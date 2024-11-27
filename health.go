// health.go
package main

import (
    "math"
    "runtime"
    "time"
)

// Track server start time
var start = time.Now()

// MB represents megabytes in bytes
const MB float64 = 1.0 * 1024 * 1024

// HealthStats holds server health information
type HealthStats struct {
    Uptime               int64   `json:"uptime"`
    AllocatedMemory      float64 `json:"allocatedMemory"`
    TotalAllocatedMemory float64 `json:"totalAllocatedMemory"`
    Goroutines           int     `json:"goroutines"`
    GCCycles             uint32  `json:"completedGCCycles"`
    NumberOfCPUs         int     `json:"cpus"`
    HeapSys              float64 `json:"maxHeapUsage"`
    HeapAllocated        float64 `json:"heapInUse"`
    ObjectsInUse         uint64  `json:"objectsInUse"`
    OSMemoryObtained     float64 `json:"OSMemoryObtained"`
}

// GetHealthStats returns current server health metrics
func GetHealthStats() *HealthStats {
    mem := &runtime.MemStats{}
    runtime.ReadMemStats(mem)

    return &HealthStats{
        Uptime:               time.Now().Unix() - start.Unix(),
        AllocatedMemory:      toMegaBytes(mem.Alloc),
        TotalAllocatedMemory: toMegaBytes(mem.TotalAlloc),
        Goroutines:           runtime.NumGoroutine(),
        NumberOfCPUs:         runtime.NumCPU(),
        GCCycles:             mem.NumGC,
        HeapSys:              toMegaBytes(mem.HeapSys),
        HeapAllocated:        toMegaBytes(mem.HeapAlloc),
        ObjectsInUse:         mem.Mallocs - mem.Frees,
        OSMemoryObtained:     toMegaBytes(mem.Sys),
    }
}

// toMegaBytes converts bytes to megabytes with precision
func toMegaBytes(bytes uint64) float64 {
    return toFixed(float64(bytes)/MB, 2)
}

// round performs mathematical rounding
func round(num float64) int {
    return int(num + math.Copysign(0.5, num))
}

// toFixed rounds a number to a specified precision
func toFixed(num float64, precision int) float64 {
    output := math.Pow(10, float64(precision))
    return float64(round(num*output)) / output
}
