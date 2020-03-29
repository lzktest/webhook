package main
import(
	"flag"
	"fmt"
	"path"
	"io/ioutil"
	"log"
	"net/http"
	"time"
//"strconv"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"github.com/bitly/go-simplejson"	
	daemon "github.com/sevlyar/go-daemon"
)
var (
	help = flag.Bool("h",false,"The Help")
	signal=flag.String("s", "", "send `signal` to a master process: stop, reload")
	port = flag.String("p", "7442","HTTP Server Port,Default `7442`")
)
var timecont int
type confdata struct{
	owner string
	projectname string
	branch string
	password string
	shellPath string
}
var cfdata []confdata

func init(){
	timecont=0
	flag.Usage = usage
	loadconffile()
}
func usage(){
	fmt.Fprintf(os.Stderr, `Webhook Vsersion: ebhook/1.0.0 
	Usage: hook [-h] [-s signal] [-p port]
	Options:
	`)
	flag.PrintDefaults()
}
func killHandler(sig os.Signal) error {
	log.Println("已停止运行")
	return nil
}
func reloadHandler(sig os.Signal) error {
	log.Println("服务器重载成功")
	return nil
}
/*
*   main主函数入口
*/
func main() {
	//命令行参数获取
	flag.Parse()
	if *help {
		flag.Usage()
	}else{
		daemonHTTP()
	}
	
}
//计时器  定时重置访问次数
func timecount(i *int){
	c := time.Tick(3*time.Second)
        	for _= range c {
		*i=0
        }
}

func daemonHTTP(){
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGKILL, killHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"),syscall.SIGHUP, reloadHandler)
	cntxt := &daemon.Context{
			PidFileName: "logs/webhook.pid",
			PidFilePerm:0644,
			LogFileName: "logs/webhook.log",
			LogFilePerm:0640,
			WorkDir:	"./",
			Umask:	027,
			Args:		os.Args,
		}
	if len(daemon.ActiveFlags())>0{
		d, err := cntxt.Search()
		if err !=nil {
			log.Fatalln("信号发送失败：", err)
			}
		daemon.SendCommands(d)
		return		
	}
	d, err :=cntxt.Reborn()
	if err !=nil {
		log.Fatal("无法启动服务器：", err)
		return
	}
	if d != nil{
		return	
	}

	defer cntxt.Release()

	log.Print("-----------------")
	log.Print("进程运行")

	go serveHTTP()

	err = daemon.ServeSignals()
	if err != nil{
		log.Println("Error:", err)	
	}
	log.Println("daemon terminated")
}
// 路由规则
func serveHTTP(){
	go timecount(&timecont)
	mux := http.NewServeMux()
	mux.HandleFunc("/gitee", gitee)
	mux.HandleFunc("/gitlab", gitlab)
	mux.HandleFunc("/coding", coding)
	mux.HandleFunc("/gogs", gogs)
	mux.HandleFunc("/",index)
	log.Fatalln(http.ListenAndServe(":"+*port, mux))
}
/**
*	读取所有json文件
*/
func loadconffile()string{
	err, files:=TraverseDir(`./conf`)
	if err != nil{
		log.Print(`加载配置文件错误：` + err.Error())
		return "error"
	}else{
		for _,fp := range files{ 
			fileSuffix := path.Ext(fp)
			if fileSuffix == ".json"{
				Parserdate(fp)
			}
		}
		return "sussess"
	} 
}
func TraverseDir(dirPth string) (err error,files []string){
	dir, err := ioutil.ReadDir(dirPth)
	if err !=nil{
		return
	}
	PthSep := string(os.PathSeparator)
	//suffix=strings.ToUpper(suffix) //忽略后缀匹配的大小写
	for _, fi := range dir {
		if fi.IsDir() {//忽略目录
			TraverseDir(dirPth + PthSep + fi.Name())
		}else{
			files =append(files, dirPth+PthSep+fi.Name())
		}
	}
	return
}
func Parserdate(filePath string)(){
	f,err := os.Open(filePath)
	if err !=nil {
		log.Println("文件open错误:" +filePath +err.Error())
		return
	}
	line, err := ioutil.ReadAll(f)
	if err !=nil {
		log.Fatal("无法读取文件: "+ filePath +err.Error())
		return
	}
	//解析json
	fileJSON, err := simplejson.NewJson(line)
	if err != nil{
		log.Print(`json解析错误：`+err.Error())
		return
	}
	password, _:=fileJSON.Get(`password`).String()
	shellPath, _:=fileJSON.Get(`shellpath`).String()
	projname, _:=fileJSON.Get(`projectName`).String()
	branch, _:=fileJSON.Get(`branch`).String()
	owner, _:=fileJSON.Get(`owner`).String()
	c := confdata{
		password:password,
		shellPath:shellPath,
		projectname:projname,
		branch:branch,
		owner:owner,
	}
	cfdata =append(cfdata,c)
	return
}
/*
* 匹配分支及验证码
*/
func hook(owner string, projectName string,branch string,pwd string) (rest string) {
	i,j :=0,0
	for _,t := range cfdata{
			j++
			i=0
		if branch != t.branch{
			rest ="未匹配到分支\n"	
			i++
			break
			}
		if owner != t.owner{
			rest ="未匹配到所属者\n"
			i++
			break		
			}
		if pwd != t.password{
			rest ="未匹配到项目名称或验证码不正确\n"	
			i++
			break	
			}
		if projectName != t.projectname{
			rest ="未匹配到项目名称或验证码不正确\n"	
			i++
			break
			}
	}
	if i==0{
		c :=cfdata[j-1].shellPath
		cmd := exec.Command("sh", "-c",c)
		ouput, _ := cmd.CombinedOutput()
		err := cmd.Run() //该操作阻塞
		if err.Error() =="exec: already started" {
			rest="执行成功\n"+"output:\n" +string(ouput)		
		}else{
			rest =`shell执行异常:`+c+ ":\n"+err.Error()+"\n,output:\n" +string(ouput)
		}
	}
	
	return
}


/**
*	路由调用函数/、/gitee、/gitlab
*/
func index(w http.ResponseWriter,request *http.Request){
	w.WriteHeader(403)
	w.Write([]byte("403"))
}


func gitee(w http.ResponseWriter, request *http.Request){
	timecont++
	if timecont<20 && request.Header.Get("Content-Type") == "application/json" && request.Header.Get("User-Agent") == "git-oschina-hook" && request.Method == "POST"{
		json := ParseGitEE(request)
		w.Write([]byte(json))
	}else{
		w.WriteHeader(403)
		w.Write([]byte("403"))
	}
}


func gitlab(w http.ResponseWriter, request *http.Request){
	timecont++
	if timecont<20 && request.Header.Get("Content-Type") == "application/json" && request.Method == "POST"{
		json := ParseGitLab(request)
		w.Write([]byte(json))
	}else{
		w.WriteHeader(403)
		w.Write([]byte("403"))
	}
}
func coding(w http.ResponseWriter, request *http.Request) {
	timecont++
	if timecont<20 && request.Method == "POST" && request.Header.Get("User-Agent") == "Coding.net Hook"{
		json := ParseCoding(request)
		w.Write([]byte(json))
	}else{
		w.WriteHeader(403)
		w.Write([]byte("403"))
	}
}

func gogs(w http.ResponseWriter, request *http.Request) {
	timecont++
	if timecont<20 && request.Method == "POST"{
		json := ParseGogs(request)
		w.Write([]byte(json))
	}else{
		w.WriteHeader(403)
		w.Write([]byte("403"))
	}
}

/*
*	gitleb.com 的webhook解析  json
*/
func ParseGitLab(request *http.Request) (re string) {
	result, err := ioutil.ReadAll(request.Body)
	if err !=nil {
		log.Println(`gitlab请求参数无法获取：` + err.Error())
		re = "gitlab未获取到数据"
		return 	
	}
	// 解析json
	json, err := simplejson.NewJson(result)
	if err !=nil {
		re="gitlab无法解析json"
		return		
		}
	//获取分支名称
	ref, err := json.Get(`ref`).String()
	if err !=nil {
		re="gitlab未找到ref"
		return		
		}
	//捕获分割字符串发生的异常
	defer func() {
        err := recover() //recover内置函数可以捕获到异常
        if err != nil {  //nil是err的零值
            log.Println("err=", err)
            log.Println("gitlab分割字符串发生错误")
        	}
    	}()
	branchs := strings.Split(ref, `/`)
	branch := branchs[2]
	//获取项目名称
	projName, err := json.Get(`project`).Get(`path_with_namespace`).String()
	if err !=nil {
		log.Println("gitlab获取项目名称失败"+err.Error())
		re="获取项目名称失败"
		return
	}	
	projectNameArr := strings.Split(projName, `/`)
	owner :=projectNameArr[0]
	projectName := projectNameArr[1]
	//获取密码
	pwd, err :=json.Get(`checkout_sha`).String()
	if err !=nil {
		log.Println("gitlab获取验证码失败"+err.Error())
		return
	}
	re =hook(owner, projectName, branch, pwd)
	return 
}


/**
*	gitee.com 的webhook解析 目前content-type只有json格式   gitee.com和gite
*/
func ParseGitEE(request *http.Request) (re string) {
	result, err := ioutil.ReadAll(request.Body)
	if err !=nil {
		log.Println(`gitee请求参数无法获取：` + err.Error())
		re = "gitee未获取到数据"
		return 	
	}
	// 解析json
	json, err := simplejson.NewJson(result)
	if err !=nil {
		re="gitee无法解析json"
		return		
		}
	//获取分支名称
	ref, err := json.Get(`ref`).String()
	if err !=nil {
		re="gitee未找到ref"
		return		
		}
	//捕获分割字符串发生的异常
	defer func() {
        err := recover() //recover内置函数可以捕获到异常
        if err != nil {  //nil是err的零值
            log.Println("err=", err)
            log.Println("gitee分割字符串发生错误")
        	}
    	}()
	branchs := strings.Split(ref, `/`)
	branch := branchs[2]
	//获取项目名称
	projName, err := json.Get(`repository`).Get(`path_with_namespace`).String()
	if err !=nil {
		log.Println("gitlab获取项目名称失败"+err.Error())
		re="获取项目名称失败"
		return
	}	
	projectNameArr := strings.Split(projName, `/`)
	owner :=projectNameArr[0]
	projectName := projectNameArr[1]
	//获取密码
	pwd, err :=json.Get(`password`).String()
	if err !=nil {
		log.Println("gitee获取验证码失败"+err.Error())
		return
	}
	re =hook(owner, projectName, branch, pwd)
	return 
}
/**
*
* 解析coding.net的数据
*
**/
func ParseCoding(request *http.Request) (re string) {
	result, err := ioutil.ReadAll(request.Body)
	if err !=nil {
		log.Println(`coding请求参数无法获取：` + err.Error())
		re = "coding未获取到数据"
		return 	
	}
	// 解析json
	json, err := simplejson.NewJson(result)
	if err !=nil {
		re="coding无法解析json"
		return		
		}
	//获取分支名称
	ref, err := json.Get(`ref`).String()
	if err !=nil {
		re="coding未找到ref"
		return		
		}
	//捕获分割字符串发生的异常
	defer func() {
        err := recover() //recover内置函数可以捕获到异常
        if err != nil {  //nil是err的零值
            log.Println("err=", err)
            log.Println("coding分割字符串发生错误")
        	}
    	}()
	branchs := strings.Split(ref, `/`)
	branch := branchs[2]
	//获取项目名称
	projName, err := json.Get(`repository`).Get(`full_name`).String()
	if err !=nil {
		log.Println("coding获取项目名称失败"+err.Error())
		re="获取项目名称失败"
		return
	}	
	projectNameArr := strings.Split(projName, `/`)
	owner :=projectNameArr[0]
	projectName := projectNameArr[1]
	//获取密码
	pwd, err :=json.Get(`repository`).Get(`owner`).Get(`login`).String()
	if err !=nil {
		log.Println("coding获取验证码失败"+err.Error())
		return
	}
	re =hook(owner, projectName, branch, pwd)
	return 
}

/**
*
* 解析coding.net的数据
*
**/
func ParseGogs(request *http.Request) (re string) {
	result, err := ioutil.ReadAll(request.Body)
	if err !=nil {
		log.Println(`coding请求参数无法获取：` + err.Error())
		re = "coding未获取到数据"
		return 	
	}
	// 解析json
	json, err := simplejson.NewJson(result)
	if err !=nil {
		re="coding无法解析json"
		return		
		}
	//获取分支名称
	ref, err := json.Get(`ref`).String()
	if err !=nil {
		re="coding未找到ref"
		return		
		}
	//捕获分割字符串发生的异常
	defer func() {
        err := recover() //recover内置函数可以捕获到异常
        if err != nil {  //nil是err的零值
            log.Println("err=", err)
            log.Println("coding分割字符串发生错误")
        	}
    	}()
	branchs := strings.Split(ref, `/`)
	branch := branchs[2]
	//获取项目名称
	projName, err := json.Get(`repository`).Get(`full_name`).String()
	if err !=nil {
		log.Println("coding获取项目名称失败"+err.Error())
		re="获取项目名称失败"
		return
	}	
	projectNameArr := strings.Split(projName, `/`)
	owner :=projectNameArr[0]
	projectName := projectNameArr[1]
	//获取密码
	/* pwd, err :=json.Get(`repository`).Get(`owner`).Get(`login`).String()
	if err !=nil {
		log.Println("coding获取验证码失败"+err.Error())
		return
	}*/
	pwd := ``
	re =hook(owner, projectName, branch, pwd)
	return 
}
