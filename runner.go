package runner

import (
	"sync"
	"os"
	"github.com/andonitdeveloper/log"
	"time"
	"os/signal"
	"syscall"
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
)

const(
	PREDEV = "predev"
	DEV = "dev"
	INT = "int"
	STG = "stg"
	PROD = "prod"
)

var logger *log.Logger
var env *string
var configfile *string
var mutex = &sync.Mutex{}
var config []byte
var settings map[string]interface{}


func init(){
	logger = log.New()
	env = flag.String("env", "predev", "运行环境参数")
	configfile = flag.String("config", "", "配置文件路径")
	flag.Parse()
	*env = strings.ToLower(*env)

	if *env!= PREDEV && *env != DEV && *env != INT && *env != STG && *env != PROD{
		panic("运行环境参数错误: " + *env)
	}

	if len(*configfile) == 0{
		*configfile = "application"
		if *env != PREDEV {
			*configfile = *configfile + "-" + *env
		}

		*configfile = *configfile + ".yaml"
	}
}

func Settings() map[string]interface{}{
	mutex.Lock()
	defer mutex.Unlock()

	if config == nil{
		loadConfig()
	}

	settings := map[string]interface{}{}
	err := yaml.Unmarshal(config, &settings)
	if err != nil{
		panic("配置文件不是有效的YAML格式文件")
	}

	return settings

}
func SettingsS(out interface{}){
	mutex.Lock()
	defer mutex.Unlock()

	if config == nil{
		loadConfig()
	}

	err := yaml.Unmarshal(config, out)
	if err != nil{
		panic("配置文件不是有效的YAML格式文件")
	}
}

func loadConfig(){
	if exist(*configfile){
		b, err := ioutil.ReadFile(*configfile)
		config = b
		if err != nil{
			panic(err)
		}

	}else{
		logger.Warnf("找不到配置文件：%s", *configfile)

		config = []byte{}
	}
}

func exist(filename string) (bool){
	if _, err := os.Stat(filename); os.IsNotExist(err){
		return false;
	}

	return true;
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