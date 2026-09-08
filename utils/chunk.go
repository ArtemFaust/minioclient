package utils

import "minioclient/global"

/*
Функция разбивает разбивает массив на подмассивы заданной длины
и вовращает двумерный массив с массивами указанной длины
*/
func ChunkBy(items []global.BucketObject, chunkSize int) (chunks [][]global.BucketObject) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}

	return append(chunks, items)
}
