package main

import (
	"fmt"
	"sync"
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

// pipeline
func TestPipeLine(t *testing.T) {

	repeat := func(done <-chan struct{}, values ...string) <-chan interface{} {
		c := make(chan interface{})
		go func() {
			defer close(c)

			for {
				for _, value := range values {
					fmt.Println(value)
					select {
					case <-done:
						return
					case c <- value:
					}
				}
			}
		}()
		return c
	}

	take := func(done <-chan struct{}, valueStream <-chan interface{}, num int) <-chan interface{} {
		c := make(chan interface{})
		go func() {
			defer close(c)

			for i := 0; i < num; i++ {
				select {
				case <-done:
					return
				case v := <-valueStream:
					c <- v
				}
			}
		}()

		return c
	}

	toString := func(done <-chan struct{}, valueStream <-chan interface{}) <-chan string {
		c := make(chan string)
		go func() {
			defer close(c)

			for value := range valueStream {
				select {
				case <-done:
					return
				case c <- value.(string):
				}

			}
		}()

		return c
	}

	done := make(chan struct{})
	defer close(done)

	var message string
	for value := range toString(done, take(done, repeat(done, "I", "am."), 10)) {
		fmt.Println(value)
		message += value
	}
	fmt.Println(message)
}

// 扇出、扇入
func TestFanInAndFanOut(t *testing.T) {

	repeat := func(done <-chan struct{}, values ...string) <-chan interface{} {
		c := make(chan interface{})
		go func() {
			defer close(c)

			for {
				for _, value := range values {
					select {
					case <-done:
						return
					case c <- value:
					}
				}
			}
		}()
		return c
	}
	// 扇出
	fanOut := func(done <-chan struct{}, index int, inputStream <-chan interface{}) <-chan interface{} {
		c := make(chan interface{})

		go func() {
			defer close(c)

			for input := range inputStream {
				select {
				case <-done:
					return
				default:
				}

				inputIntValue := input.(string)
				c <- fmt.Sprintf("%s_%d|", inputIntValue, index)
			}
		}()
		return c
	}

	fanIn := func(done <-chan struct{}, inputStreams ...<-chan interface{}) <-chan interface{} {
		mergeStream := make(chan interface{})

		var wg sync.WaitGroup
		wg.Add(len(inputStreams))

		for _, inputStream := range inputStreams {
			go func(itemStream <-chan interface{}) {
				defer wg.Done()

				for value := range itemStream {
					select {
					case <-done:
						return
					default:
					}
					mergeStream <- value
				}
			}(inputStream)
		}

		go func() {
			defer close(mergeStream)
			wg.Wait()
		}()

		return mergeStream
	}

	take := func(done <-chan struct{}, valueStream <-chan interface{}, num int) <-chan interface{} {
		c := make(chan interface{})
		go func() {
			defer close(c)

			for i := 0; i < num; i++ {
				select {
				case <-done:
					return
				case v := <-valueStream:
					c <- v
				}
			}
		}()

		return c
	}

	done := make(chan struct{})
	defer close(done)

	originStream := repeat(done, "I", "am.")
	// 扇出
	var fanOutStreams []<-chan interface{}
	for i := 0; i < 3; i++ {
		fanOutStreams = append(fanOutStreams, fanOut(done, i, originStream))
	}
	// 扇入
	fanInStream := fanIn(done, fanOutStreams...)

	var message string
	for value := range take(done, fanInStream, 10) {
		//fmt.Println(value)
		message += value.(string)
	}
	fmt.Println(message)
}
