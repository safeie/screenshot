package main

import (
  "crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
)

const (
	DEF_PORT   int  = 9464
	DEF_DELAY  int  = 1
	DEF_WIDTH  int  = 1024
	DEF_HEIGHT int  = 768
	DEF_DEBUG  bool = false

	PHANTOMJS string = "/usr/local/bin/phantomjs" // PhantomJS执行路径
	SNAP_JS   string = "util/rasterize.js"        // 截图脚本路径
)

type config struct {
	port   int  // screenShot服务端口
	delay  int  // 截图延迟
	width  int  // 屏幕宽度
	height int  // 屏幕高度
	debug  bool // 是否开启调试模式
}

var conf *config // 创建全局变量 conf

func main() {
	// 初始化配置
	conf = new(config)
	flag.IntVar(&conf.port, "port", DEF_PORT, "TCP port number to listen on (default: "+strconv.Itoa(DEF_PORT)+")")
	flag.IntVar(&conf.delay, "delay", DEF_DELAY, "Delay second before shot (default: "+strconv.Itoa(DEF_DELAY)+")")
	flag.IntVar(&conf.width, "width", DEF_WIDTH, "Screen width (default: "+strconv.Itoa(DEF_WIDTH)+")")
	flag.IntVar(&conf.height, "height", DEF_HEIGHT, "Screen height (default: "+strconv.Itoa(DEF_HEIGHT)+")")
	flag.BoolVar(&conf.debug, "debug", DEF_DEBUG, "Open debug mode (default: false)")
	flag.Parse()
	//为phantomJS配置环境变量
	os.Setenv("LIBXCB_ALLOW_SLOPPY_LOCK", "1")
	os.Setenv("DISPLAY", ":0")
	// 开始服务
	http.HandleFunc("/", handler)
	log.Fatalln("ListenAndServe: ", http.ListenAndServe(":"+strconv.Itoa(conf.port), nil))
}

// 处理请求
func handler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Server", "GWS")
	var url = req.FormValue("url")
	var width = req.FormValue("width")
	var height = req.FormValue("height")
	var delay = req.FormValue("delay")
	var flush = req.FormValue("flush")
	var validURL = regexp.MustCompile(`^http(s)?://.*$`)
	if ok := validURL.MatchString(url); !ok {
		fmt.Fprintf(rw, "<html><body>请输入需要截图的网址：<form><input name=url><input type=submit value=shot></form></body></html>")
	} else {
		pic := GetPicPath(url)
		// 如果有range,表明是分段请求，直接处理
		if v := req.Header.Get("Range"); v != "" {
			http.ServeFile(rw, req, pic)
		}
		// 判断图片是否重新生成
		if i, _ := strconv.Atoi(flush); i == 1 || IsExist(pic) == false {
			pic, err := exec(url, pic, width, height, delay)
			if err != nil {
				if conf.debug == true {
					log.Println("Snapshot Error:", url, err.Error())
				}
				fmt.Fprintf(rw, "shot error: %s", err.Error())
				return
			}
			if conf.debug == true {
				log.Println("Snapshot Successful:", url, pic)
			}
		}
		http.ServeFile(rw, req, pic)
	}
	return
}

// 执行截图
func exec(url, pic, width, height, delay string) (string, error) {
	if url == "" {
		return "", errors.New("url is none.")
	}
	if width == "" {
		width = strconv.Itoa(conf.width)
	}
	if height == "" {
		height = strconv.Itoa(conf.height)
	}
	if delay == "" {
		delay = strconv.Itoa(conf.delay)
	}
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	procAttr.Dir = os.Getenv("PWD")
	procAttr.Env = os.Environ()
	dir, err := GetDir()
	var args []string
	args = make([]string, 7)
	args[0] = PHANTOMJS
	args[1] = dir + "/" + SNAP_JS
	args[2] = url
	args[3] = pic
	args[4] = delay
	args[5] = width
	args[6] = height
	process, err := os.StartProcess(PHANTOMJS, args, procAttr)
	if err != nil {
		if conf.debug == true {
			log.Println("PhantomJS start failed:" + err.Error())
		}
		return "", err
	}
	waitMsg, err := process.Wait()
	if err != nil {
		if conf.debug == true {
			log.Println("PhantomJS start wait error:" + err.Error())
		}
		return "", err
	}
	if conf.debug == true {
		log.Println(waitMsg)
	}
	return args[3], nil
}

// 根据url获取图片路径
func GetPicPath(url string) string {
	h := md5.New()
	h.Write([]byte(url))
	pic := hex.EncodeToString(h.Sum(nil))
	dir, _ := GetDir()
	path := dir + "/data/"
	os.Mkdir(path, 0755)
	path += string(pic[0:2]) + "/"
	os.Mkdir(path, 0755)
	return path + pic + ".png"
}

// 获取程序运行的目录
func GetDir() (string, error) {
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// 判断一个文件或目录是否存在
func IsExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	// Check if error is "no such file or directory"
	if _, ok := err.(*os.PathError); ok {
		return false
	}
	return false
}

// 判断一个文件或目录是否有写入权限
func IsWritable(path string) bool {
	err := syscall.Access(path, syscall.O_RDWR)
	if err == nil {
		return true
	}
	// Check if error is "no such file or directory"
	if _, ok := err.(*os.PathError); ok {
		return false
	}
	return false
}
