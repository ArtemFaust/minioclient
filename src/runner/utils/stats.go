package utils

import (
	"fmt"
	"runtime"
	"time"
)

// Метод печати данных работы worker
func PrintStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Расчет реальной физической памяти, удерживаемой рантаймом Go в данный момент
	currentRSS := m.Sys - m.HeapReleased
	fmt.Printf(`

--- ПОТРЕБЛЕНИЕ РЕССУРСОВ ---
ALLOC (живые объекты в куче сейчас) = %v MiB
CURRENT RSS (реальный объем у ОС)     = %v MiB
SYS (всего зарезервировано у ОС)    = %v MiB
TOTAL ALLOC (выделено за всё время)  = %v MiB

--- ПОТОКИ И ГОРУТИНЫ ---
Goroutines (активные горутины)     = %v
OS Threads (системные потоки)       = %v

--- СБОРКА МУСОРА (GC) ---
NextGC (порог следующей сборки)     = %v MiB
LastGC (время с последней сборки)   = %s
NumGC (всего циклов сборки мусора)  = %v
PauseTotalNs (суммарное время паузы)= %v ms
GCCPUFraction (доля CPU на GC)      = %.2f%%

--- ДЕТАЛИЗАЦИЯ КУЧИ ---
HeapIdle (свободная память в куче)  = %v MiB
HeapInuse (занятая память в куче)   = %v MiB
HeapReleased (возвращено ОС)        = %v MiB
`,
		m.Alloc/1024/1024,
		currentRSS/1024/1024, // Реальное текущее потребление процесса
		m.Sys/1024/1024,
		m.TotalAlloc/1024/1024,
		runtime.NumGoroutine(),
		runtime.NumCgoCall(), // Приблизительная оценка потоков или вызовов (альтернативно pprof)
		m.NextGC/1024/1024,
		time.Since(time.Unix(0, int64(m.LastGC))),
		m.NumGC,
		m.PauseTotalNs/1e6, // переводим наносекунды в миллисекунды
		m.GCCPUFraction*100,
		m.HeapIdle/1024/1024,
		m.HeapInuse/1024/1024,
		m.HeapReleased/1024/1024,
	)
}
