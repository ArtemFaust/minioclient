package memdb

import (
	"bytes"
	"compress/gzip"
	"io"
)

// Методы сжатия json строки
func Compress(input []byte) ([]byte, error) {
	var buf bytes.Buffer                                     // Буфер хранения сжатых данных
	gw, e := gzip.NewWriterLevel(&buf, gzip.BestCompression) // Интерфейс gzip архиватора с максимальной степенью сжатия
	if e != nil {
		return nil, e
	}
	// Выполняем сжатие данных
	if _, e := gw.Write(input); e != nil {
		return nil, e
	}
	// Обязательно закрываем writer, чтобы сбросить буфер и дописать хвост gzip
	if e := gw.Close(); e != nil {
		return nil, e
	}

	return buf.Bytes(), nil
}

// Метод восстановления исходной строки
func Decompress(input []byte) ([]byte, error) {
	// Считываем сжатые данные
	gr, e := gzip.NewReader(bytes.NewReader(input))
	if e != nil {
		return nil, e
	}
	defer gr.Close()

	// Возвращаем данные после декомпрессии
	return io.ReadAll(gr)
}
