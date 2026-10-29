package slave

import "sync/atomic"

// Глобальные данные работы воркера
var CurrentTaskId atomic.Uint64 // Текучий ID выполняемой задачи
var TaskStatus string           // Текучий статус выполняемой задачи
var Status atomic.Bool          // Текучий статус занятости воркера
