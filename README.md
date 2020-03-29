# webhook
主要用于gitee和gitlab，gogs及coding未做测试
## 部署说明
默认端口为7442
可使用-p 指定端口

## 主要操作
|参数|含义|
|---|---|
|-s reload|重新加载配置|
|-s stop|停止运行|
|-h|帮助|
|-p|指定端口|
## 目录说明
conf 目录下所有以.json的文件将视为配置文件，一个文件对应一个项目（需要手动创建）
**格式如下：**
```
{
	"owner":"my", //所属者
	"branch":"origin",  项目分支
	"projectName":"testproj",  项目名称
	"shellpath":"./test.sh",  执行脚本，绝对目录与相对目录均可，相对目录为webhook可执行文件目录为起始
	"password":"Aa123456"
}
```
log 目录为日志及pid目录（程序可自行创建）
