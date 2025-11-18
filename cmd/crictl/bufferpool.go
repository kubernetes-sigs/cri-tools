/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"sync"
)

// jsonBufferPool is a pool of bytes.Buffer instances used for JSON operations.
// This reduces GC pressure by reusing buffers instead of allocating new ones
// for each JSON indentation operation.
var jsonBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// getJSONBuffer retrieves a buffer from the pool.
// The caller must call putJSONBuffer when done to return it to the pool.
func getJSONBuffer() *bytes.Buffer {
	buf, ok := jsonBufferPool.Get().(*bytes.Buffer)
	if !ok {
		return new(bytes.Buffer)
	}

	return buf
}

// putJSONBuffer resets and returns a buffer to the pool.
// This should be called with defer after getJSONBuffer.
func putJSONBuffer(buf *bytes.Buffer) {
	buf.Reset()
	jsonBufferPool.Put(buf)
}
