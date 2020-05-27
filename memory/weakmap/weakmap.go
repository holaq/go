// Author: Zwei.Ren
package weakmap

import (
	"fmt"
	"sync"
)

type Map interface {
	Load(key interface{}) (value interface{}, ok bool)
	Store(key, value interface{})
	LoadOrStore(key, value interface{}) (actual interface{}, loaded bool)
	Delete(key interface{})
	Range(f func(key, value interface{}) bool)
}

// 通过链表来实现
type entry struct {
	k, o interface{}
	p, n *entry
}

// 一个有限制最大长度的Map
// 超出长度时，抛弃最久没有操作的key
// "操作"是指写入、读取
type weakMap struct {
	sync.Mutex

	Limit int

	count int

	m    map[interface{}]*entry
	h, t *entry
}

func NewSyncMap() Map {
	return new(sync.Map)
}

func NewWeakMap(limit int) Map {
	return &weakMap{
		Limit: limit,
		count: 0,
		m:     make(map[interface{}]*entry, limit),
	}
}

func (m *weakMap) KeysLink() []string {
	m.Lock()
	defer m.Unlock()

	res := make([]string, m.count)
	if m.h != nil {
		ind := 0
		e := m.h
		for e != nil {
			res[ind] = fmt.Sprintf("%v", e.k)
			ind++
			e = e.n
		}
	}
	return res
}

func (m *weakMap) remove(e *entry) {
	if e.p == nil { // 移除的是第一个
		if e.n == nil { // 唯一一个
			m.t = nil
		} else { // 后面有，需要将其作为第一个
			e.n.p = nil
		}
		m.h = e.n // 修改头部信息
	} else if e.n == nil { // 移除的是最后一个
		e.p.n = nil
		m.t = e.p // 修改尾部信息
	} else { // 移除的是中间的
		e.p.n = e.n // 将头尾两个连接起来
		e.n.p = e.p
	}
	e.o, e.p, e.n = nil, nil, nil
	m.count--
}

func (m *weakMap) activate(e *entry) {
	if e.p == nil { // 本来就在第一个
		// 无需操作
	} else if e.n == nil { // 本来在最后一个
		e.p.n = nil // 上一个变成最后一个
		m.t = e.p   // 倒数2设为表尾

		e.p = nil // 自己上一个为空
		e.n = m.h // 自己的下一个为原来的第一个
		m.h.p = e // 原来的第一个的上一个是自己
		m.h = e   // 成为表头
	} else { // 在中间
		e.p.n = e.n // 连接头尾两个
		e.n.p = e.p

		e.n = m.h // 自己的下一个为原来的第一个
		m.h.p = e // 原来的第一个的上一个是自己
		m.h = e   // 成为表头
	}
}

func (m *weakMap) insert(e *entry) {
	if m.h == nil {
		m.h, m.t = e, e
	} else {
		e.n = m.h
		m.h.p = e
		m.h = e
	}
	if m.count < m.Limit {
		m.count++
	} else if t := m.t; t != nil { // 删除最后一个
		if t.p != nil {
			t.p.n = nil
		}
		m.t = t.p
		delete(m.m, t.k)
	}
}

func (m *weakMap) Load(key interface{}) (value interface{}, ok bool) {
	m.Lock()
	defer m.Unlock()

	var e *entry
	if e, ok = m.m[key]; ok {
		m.activate(e) // 如果读取到了，就激活
		value = e.o
	}
	return
}

func (m *weakMap) Store(key, value interface{}) {
	m.Lock()
	defer m.Unlock()

	if e, ok := m.m[key]; ok { // 如果读取到了，就激活
		e.o = value
		m.activate(e)
	} else { // 如果不存在，插入
		e = &entry{k: key, o: value}
		m.insert(e)
		m.m[key] = e
	}
}

func (m *weakMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	m.Lock()
	defer m.Unlock()

	if e, ok := m.m[key]; ok { // 如果读取到了，就激活，不用覆盖值
		m.activate(e)
		actual, loaded = e.o, true
	} else { // 如果不存在，插入
		e = &entry{k: key, o: value}
		m.insert(e)
		m.m[key] = e
		actual = value
	}
	return
}

func (m *weakMap) Delete(key interface{}) {
	m.Lock()
	defer m.Unlock()

	if e, ok := m.m[key]; ok { // 这个key有值
		m.remove(e)
		delete(m.m, key)
	}
}

// 遍历不修改任何链表
func (m *weakMap) Range(f func(key, value interface{}) bool) {
	m.Lock()
	defer m.Unlock()

	for key, value := range m.m {
		if !f(key, value.o) {
			return
		}
	}
}

func CopyMap(from Map, to map[interface{}]interface{}) {
	if from == nil || to == nil {
		return
	}
	if weak, is := from.(*weakMap); is {
		weak.Lock()
		defer weak.Unlock()
		for key, value := range weak.m {
			to[key] = value.o
		}
	} else {
		from.Range(func(k, v interface{}) bool {
			to[k] = v
			return true
		})
	}
}

func Keys(m Map) []interface{} {
	if m == nil {
		return []interface{}{}
	}
	if weak, is := m.(*weakMap); is {
		weak.Lock()
		defer weak.Unlock()
		res := make([]interface{}, 0, weak.count)
		for key, _ := range weak.m {
			res = append(res, key)
		}
		return res
	} else {
		res := make([]interface{}, 0, 100)
		m.Range(func(k, _ interface{}) bool {
			res = append(res, k)
			return true
		})
		return res
	}
}

func Values(m Map) []interface{} {
	if m == nil {
		return []interface{}{}
	}
	if weak, is := m.(*weakMap); is {
		weak.Lock()
		defer weak.Unlock()
		res := make([]interface{}, 0, weak.count)
		for _, val := range weak.m {
			res = append(res, val)
		}
		return res
	} else {
		res := make([]interface{}, 0, 100)
		m.Range(func(_, v interface{}) bool {
			res = append(res, v)
			return true
		})
		return res
	}
}

func StringKeys(m Map) []string {
	if m == nil {
		return []string{}
	}
	if weak, is := m.(*weakMap); is {
		weak.Lock()
		defer weak.Unlock()
		res := make([]string, 0, weak.count)
		for key, _ := range weak.m {
			res = append(res, key.(string))
		}
		return res
	} else {
		res := make([]string, 0, 100)
		m.Range(func(k, _ interface{}) bool {
			res = append(res, k.(string))
			return true
		})
		return res
	}
}
