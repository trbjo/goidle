package main

import (
    "fmt"
    "sync"
    "syscall"

    "github.com/rajveermalviya/go-wayland/wayland/client"
    "github.com/rajveermalviya/go-wayland/wayland/staging/ext-session-lock-v1"
)

type LockManager struct {
    display            *client.Display
    registry           *client.Registry
    compositor         *client.Compositor
    subcompositor      *client.Subcompositor
    shm                *client.Shm
    sessionLockManager *ext_session_lock.ExtSessionLockManager
    sessionLock        *ext_session_lock.ExtSessionLock
    surfaces           map[*client.Output]*ext_session_lock.ExtSessionLockSurface
    outputs            []*client.Output
    locked             bool
    runDisplay         bool
    mu                 sync.Mutex
}

func (lm *LockManager) displayRoundTrip() {
    callback, err := lm.display.Sync()
    if err != nil {
        lg.Error("unable to get sync callback", err)
        return
    }
    defer callback.Destroy()

    done := false
    callback.SetDoneHandler(func(_ client.CallbackDoneEvent) {
        done = true
    })

    for !done {
        lm.display.Context().Dispatch()
    }
}

func NewLockManager(display *client.Display) (*LockManager, error) {
    registry, err := display.GetRegistry()
    if err != nil {
        return nil, fmt.Errorf("unable to get registry: %w", err)
    }

    lm := &LockManager{
        display:  display,
        registry: registry,
        surfaces: make(map[*client.Output]*ext_session_lock.ExtSessionLockSurface),
    }

    registry.SetGlobalHandler(lm.handleGlobal)

    lm.displayRoundTrip()

    if lm.compositor == nil || lm.subcompositor == nil || lm.shm == nil || lm.sessionLockManager == nil {
        return nil, fmt.Errorf("missing required Wayland interfaces")
    }

    return lm, nil
}

func (lm *LockManager) handleGlobal(e client.RegistryGlobalEvent) {
    switch e.Interface {
    case "wl_compositor":
        lm.compositor = client.NewCompositor(lm.display.Context())
        lm.registry.Bind(e.Name, e.Interface, e.Version, lm.compositor)
    case "wl_subcompositor":
        lm.subcompositor = client.NewSubcompositor(lm.display.Context())
        lm.registry.Bind(e.Name, e.Interface, e.Version, lm.subcompositor)
    case "wl_shm":
        lm.shm = client.NewShm(lm.display.Context())
        lm.registry.Bind(e.Name, e.Interface, e.Version, lm.shm)
    case "ext_session_lock_manager_v1":
        lm.sessionLockManager = ext_session_lock.NewExtSessionLockManager(lm.display.Context())
        lm.registry.Bind(e.Name, e.Interface, e.Version, lm.sessionLockManager)
    case "wl_output":
        output := client.NewOutput(lm.display.Context())
        lm.registry.Bind(e.Name, e.Interface, e.Version, output)
        lm.outputs = append(lm.outputs, output)
    }
}

func (lm *LockManager) createLockSurfaces() {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    for _, output := range lm.outputs {
        surface := client.NewSurface(lm.display.Context())

        // Set up the surface
        surface.SetBufferScale(1) // Set the scale factor to 1

        // Create a region that covers the entire surface
        region, err := lm.compositor.CreateRegion()
        defer region.Destroy()
        region.Add(0, 0, 1000000, 1000000) // Large values to cover any possible size
        surface.SetInputRegion(region)

        // Now create the lock surface
        lockSurface, err := lm.sessionLock.GetLockSurface(surface, output)
        if err != nil {
            fmt.Printf("Failed to create lock surface: %v\n", err)
            surface.Destroy()
            continue
        }

        lockSurface.SetConfigureHandler(func(e ext_session_lock.ExtSessionLockSurfaceConfigureEvent) {
            fmt.Printf("Lock surface configured: %dx%d\n", e.Width, e.Height)

            // Create and attach a buffer of the correct size
            lm.createAndAttachBuffer(surface, int(e.Width), int(e.Height))

            // Acknowledge the configure event
            lockSurface.AckConfigure(e.Serial)
        })

        lm.surfaces[output] = lockSurface
    }
}

func (lm *LockManager) createAndAttachBuffer(surface *client.Surface, width, height int) {
    stride := width * 4
    size := stride * height

    // Create shared memory
    fd, err := createAnonymousFile(size)
    if err != nil {
        fmt.Printf("Failed to create anonymous file: %v\n", err)
        return
    }
    defer syscall.Close(fd)

    data, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
    if err != nil {
        fmt.Printf("Failed to mmap: %v\n", err)
        return
    }
    defer syscall.Munmap(data)

    // Fill with a solid color (e.g., dark gray)
    for i := 0; i < len(data); i += 4 {
        data[i] = 64    // Blue
        data[i+1] = 64  // Green
        data[i+2] = 64  // Red
        data[i+3] = 255 // Alpha
    }

    // Create a buffer from the shared memory
    pool, err := lm.shm.CreatePool(fd, int32(size))
    if err != nil {
        fmt.Printf("Failed to create pool: %v\n", err)
        return
    }
    defer pool.Destroy()

    buffer, err := pool.CreateBuffer(0, int32(width), int32(height), int32(stride), uint32(client.ShmFormatArgb8888))
    if err != nil {
        fmt.Printf("Failed to create buffer: %v\n", err)
        return
    }

    // Attach the buffer to the surface and commit
    surface.Attach(buffer, 0, 0)
    surface.Damage(0, 0, int32(width), int32(height))
    surface.Commit()
}

func createAnonymousFile(size int) (int, error) {
    return 0, nil
}

func (lm *LockManager) Lock() {
    var err error
    lm.sessionLock, err = lm.sessionLockManager.Lock()
    if err != nil {
        fmt.Println("failed to create lock: %w", err)
        return
    }

    lm.sessionLock.SetLockedHandler(func(e ext_session_lock.ExtSessionLockLockedEvent) {
        fmt.Println("Session locked")
        lm.locked = true
        lm.createLockSurfaces()
        fmt.Println("setup surfaces")
    })

    lm.sessionLock.SetFinishedHandler(func(e ext_session_lock.ExtSessionLockFinishedEvent) {
        fmt.Println("Lock finished")
        lm.destroyLockSurfaces()
        lm.sessionLock.Destroy()
        lm.sessionLock = nil
    })
    lm.displayRoundTrip()

}

func (lm *LockManager) Unlock() error {
    if lm.sessionLock == nil {
        return fmt.Errorf("no active lock")
    }

    err := lm.sessionLock.UnlockAndDestroy()
    if err != nil {
        return fmt.Errorf("failed to unlock: %w", err)
    }

    lm.sessionLock = nil
    return nil
}

func (lm *LockManager) destroyLockSurfaces() {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    for _, surface := range lm.surfaces {
        surface.Destroy()
    }
    lm.surfaces = make(map[*client.Output]*ext_session_lock.ExtSessionLockSurface)
}

func (lm *LockManager) Close() {
    if lm.sessionLock != nil {
        lm.sessionLock.UnlockAndDestroy()
    }
    lm.destroyLockSurfaces()
}
