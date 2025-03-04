package engine

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Monibuca/engine/v2/avformat"
)

type RingItem struct {
	avformat.AVPacket
	sync.WaitGroup
	*bytes.Buffer
	UpdateTime time.Time
}

// Ring 环形缓冲，使用数组实现
type Ring struct {
	*RingItem
	buffer []RingItem
	Size   int
	Index  int
	Flag   int32
}

// NewRing 创建Ring，传入大小指数
func NewRing(exp int) (r *Ring) {
	r = new(Ring)
	r.Size = 1 << exp
	r.buffer = make([]RingItem, r.Size)
	r.RingItem = &r.buffer[0]
	r.Add(1)
	return
}
func (r *Ring) offset(v int) int {
	return (r.Index + v) & (r.Size - 1)
}

// GoTo 移动到指定索引处
func (r *Ring) GoTo(index int) {
	r.Index = index
	r.RingItem = &r.buffer[index]
}

// GetAt 获取指定索引处的引用
func (r *Ring) GetAt(index int) *RingItem {
	return &r.buffer[index]
}

// GetNext 获取下一个位置的引用
func (r *Ring) GetNext() *RingItem {
	return &r.buffer[r.offset(1)]
}

// GetLast 获取上一个位置的引用
func (r *Ring) GetLast() *RingItem {
	return &r.buffer[r.offset(-1)]
}

// GoNext 移动到下一个位置
func (r *Ring) GoNext() {
	r.Index = r.offset(1)
	r.RingItem = &r.buffer[r.Index]
}

// GoBack 移动到上一个位置
func (r *Ring) GoBack() {
	r.Index = r.offset(-1)
	r.RingItem = &r.buffer[r.Index]
}

// NextW 写下一个
func (r *Ring) NextW() {
	item := r.RingItem
	item.UpdateTime = time.Now()
	if atomic.CompareAndSwapInt32(&r.Flag, 0, 1) {
		r.GoNext()
		r.RingItem.Add(1)
		item.Done()
		//Flag不为1代表被Dispose了，但尚未处理Done
		if !atomic.CompareAndSwapInt32(&r.Flag, 1, 0) {
			r.RingItem.Done()
		}
	}
}

// NextR 读下一个
func (r *Ring) NextR() {
	r.GoNext()
	r.Wait()
}

func (r *Ring) GetBuffer() *bytes.Buffer {
	if r.Buffer == nil {
		r.Buffer = bytes.NewBuffer([]byte{})
	} else {
		r.Reset()
	}
	return r.Buffer
}
// Clone 克隆一个Ring
func (r *Ring) Dispose() {
	current := r.RingItem
	if atomic.CompareAndSwapInt32(&r.Flag, 0, 2) {
		current.Done()
	} else if atomic.CompareAndSwapInt32(&r.Flag, 1, 2) {
		//当前是1代表正在写入，此时变成2，但是Done的任务得交给NextW来处理
	} else if atomic.CompareAndSwapInt32(&r.Flag, 0, 2) {
		current.Done()
	}
}
// Clone 克隆一个Ring
func (r Ring) Clone() *Ring {
	return &r
}

// Timeout 发布者是否超时了
func (r *Ring) Timeout() bool {
	return time.Since(r.UpdateTime) > config.PublishTimeout
}
