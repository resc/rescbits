package tools

import (
	"sort"
)

type (
	SortedMap interface {
		Iterator() Iterator
		Put(key string, value interface{}) int
		Get(key string) (interface{}, bool)
		Del(key string) (interface{}, bool)
		Len() int
	}

	sortedMap []KeyValue

	KeyValue struct {
		Key   string
		Value interface{}
	}

	Iterator func() (kv KeyValue, ok bool)
)

var _ SortedMap = (*sortedMap)(nil)

func NewSortedMap() *sortedMap {
	return newSortedMap(16)
}

func newSortedMap(cap int) *sortedMap {
	mm := make(sortedMap, 0, cap)
	return &mm
}

func NewSortedMapFrom(kvs ...KeyValue) *sortedMap {
	mm := newSortedMap(len(kvs))
	for _, kv := range kvs {
		mm.Put(kv.Key, kv.Value)
	}
	return mm
}

func NewSortedMapFromIterator(iterator Iterator) *sortedMap {
	mm := newSortedMap(16)
	for kv, ok := iterator(); ok; kv, ok = iterator() {
		mm.Put(kv.Key, kv.Value)
	}
	return mm
}

func NewSortedMapFromMap(m map[string]interface{}) *sortedMap {
	mm := newSortedMap(len(m))
	for k, v := range m {
		mm.Put(k, v)
	}
	return mm
}

func NewSortedMapFromStringMap(m map[string]string) *sortedMap {
	mm := newSortedMap(len(m))
	for k, v := range m {
		mm.Put(k, v)
	}
	return mm
}

func (m *sortedMap) Iterator() Iterator {
	mm := *m
	return func() (KeyValue, bool) {
		kv := KeyValue{}
		if len(mm) > 0 {
			kv = mm[0]
			mm = mm[1:]
			return kv, true
		}
		return kv, false
	}

}

func (m *sortedMap) Put(key string, value interface{}) int {
	mm := *m

	i := m.insertIndexOf(key)

	if i == len(mm) {
		mm = append(mm, KeyValue{Key: key, Value: value})
	} else {
		if mm[i].Key == key {
			mm[i].Value = value
		} else {
			mm = append(mm, KeyValue{})
			copy(mm[i+1:], mm[i:])
			mm[i] = KeyValue{Key: key, Value: value}
		}
	}

	*m = mm
	return i
}

func (m *sortedMap) Del(key string) (interface{}, bool) {
	mm := *m
	i := m.insertIndexOf(key)
	if i < len(mm) && mm[i].Key == key {
		val := mm[i].Value
		copy(mm[i:], mm[i+1:])
		mm[len(mm)-1] = KeyValue{}
		mm = mm[:len(mm)-1]
		*m = mm
		return val, true
	}
	return nil, false
}

func (m *sortedMap) Get(key string) (interface{}, bool) {
	mm := *m
	i := m.insertIndexOf(key)
	if i < len(mm) && mm[i].Key == key {
		return mm[i].Value, true
	}
	return nil, false
}

func (m *sortedMap) Len() int {
	return len(*m)
}

func (m *sortedMap) insertIndexOf(key string) int {
	mm := *m
	index := sort.Search(len(mm), func(i int) bool { return mm[i].Key >= key })
	return index
}

func (m *sortedMap) Less(i, j int) bool {
	return (*m)[i].Key < (*m)[j].Key
}

func (m *sortedMap) Swap(i, j int) {
	mm := *m
	mm[j], mm[i] = mm[i], mm[j]
	*m = mm
}

func (m *sortedMap) Sort() {
	sort.Stable(m)
}
