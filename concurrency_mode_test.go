package main

import (
	"fmt"
	"testing"
	"time"
)

// 防goroutine泄漏
// 如果父goroutine负责创建goroutine，也负责停止它
func TestCloseChildGo(t *testing.T) {

	doWork := func(done <-chan struct{}, strings <-chan string) <-chan struct{} {
		terminated := make(chan struct{})
		go func() {
			defer fmt.Println("Chan terminated close.")
			defer close(terminated)
			for {
				select {
				case s := <-strings:
					fmt.Println(s)
				case <-done:
					return
				}
			}
		}()
		return terminated
	}

	done := make(chan struct{})
	terminated := doWork(done, nil)
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("Chan done close.")
		close(done)
	}()
	<-terminated
	fmt.Println("Done,")
	t.Log("success")
}

// 实现：多个chan组合成一个u_chan，当某一个chan有消息时，u_chan关闭
// or-channel 模式
func TestOrChan(t *testing.T) {

	var or func(channels ...<-chan struct{}) <-chan struct{}
	or = func(c ...<-chan struct{}) <-chan struct{} {
		switch len(c) {
		case 0:
			return nil
		case 1:
			return c[0]
		}

		// 当c切片数量大于1时
		orChan := make(chan struct{})
		go func() {
			defer close(orChan)

			switch len(c) {
			case 2:
				select {
				case <-c[0]:
				case <-c[1]:
				}
			default:
				select {
				case <-c[0]:
				case <-c[1]:
				case <-c[2]:
				case <-or(append(c[3:], orChan)...):
				}
			}
		}()
		return orChan
	}

	sig := func(after time.Duration) chan struct{} {
		c := make(chan struct{})
		go func() {
			defer fmt.Printf("after %d s, chan close.\n", after/time.Second)
			defer close(c)
			time.Sleep(after)
		}()
		return c
	}

	<-or(sig(3*time.Second), sig(6*time.Second), sig(8*time.Second))
	fmt.Println("Done.")
}
