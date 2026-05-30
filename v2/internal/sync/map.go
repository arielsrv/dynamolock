/*
Copyright 2023 U. Cirello (cirello.io and github.com/cirello-io)

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

package sync

import "sync"

type Map[K comparable, V any] struct {
	syncMap sync.Map
}

func (m *Map[K, V]) Store(key K, value V) {
	m.syncMap.Store(key, value)
}

func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.syncMap.Range(func(key, value any) bool {
		k, ok1 := key.(K)
		v, ok2 := value.(V)
		if ok1 && ok2 {
			return f(k, v)
		}
		return false
	})
}

func (m *Map[K, V]) Delete(key K) {
	m.syncMap.Delete(key)
}

func (m *Map[K, V]) Load(key K) (V, bool) {
	v, ok := m.syncMap.Load(key)
	var zero V
	if v != nil {
		if val, ok2 := v.(V); ok2 {
			return val, ok
		}
	}
	return zero, ok
}
