package runner

import (
	"sync"
	"os"
	"github.com/andonitdeveloper/log"
	"time"
	"os/signal"
	"syscall"
)


var logger *log.Logger

func init(){
	logger = log.New()
}



type Runner struct{
	wait *sync.WaitGroup
	signalChan chan os.Signal
	cancelChan chan bool
	timeout time.Duration
	callbacks map[string]func()
}

func New() *Runner{
	runner := &Runner{
		timeout: 10 * time.Second,
		signalChan: make(chan os.Signal, 1),
		cancelChan: make(chan bool, 1),
		wait: &sync.WaitGroup{},
		callbacks: map[string]func() {},
	}

	signal.Notify(runner.signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	runner.wait.Add(1)
	go runner.listen()

	return runner
}

func (runner *Runner) AddShutdownListenr(listenr string, callback func()){
	if callback != nil {
		runner.callbacks[listenr] = callback
	}
}

func (runner *Runner) Shutdown(){
	select {
	case runner.cancelChan <- true:
		return
	default:
		return
	}
}

func (runner *Runner) Wait(){
	runner.wait.Wait()
	count := len(runner.callbacks)
	doneChan := make(chan string, count)
	for k, f := range runner.callbacks{
		go func(key string, callback func()) {
			callback()
			doneChan <- key
		}(k, f)
	}

	timer := time.NewTimer(runner.timeout)

	for{
		select{
		case <- timer.C:
			logger.Warnf("关闭超时，仍有%d个任务未完成", count)
			return
		case <-doneChan:
			count--
			if count == 0{
				return
			}

		}
	}
}

func (runner *Runner) listen(){
	defer runner.wait.Done()

	for{
		select{
		case sig := <- runner.signalChan:
			logger.Debug("接收到信号：" + sig.String())
			return
		case <- runner.cancelChan:
			logger.Debug("取消等待信号")
			return
		}
	}
}