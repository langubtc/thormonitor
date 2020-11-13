package main

import (
	"bytes"
	"fmt"
	"github.com/hsw409328/ip-range-lib"
	"golang.org/x/crypto/ssh"
	"log"
	"net"
	"strconv"
	"sync"
	"thormonitor/config"
	"thormonitor/monitor"
	"time"
)



type CommandResult struct {
	Target string
	Cmdout string
	Status int32
	Optype string
}



func MinerIPFunc(ipString string) []string {
	t := ip_range_lib.NewIpRangeLib()

	ipRange := ipString
	result, err := t.IpRangeToIpList(ipRange)
	if err != nil {
		log.Fatalln("error")
	}
	return result

}



// 执行命令主要函数方法
func remoteExec(user,ip,password,opType string,port int, command string) (result CommandResult) {
	//fmt.Println(command)
	var commandStr string

	// 拼接目标地址 ip:port
	sshHost := ip + ":" + strconv.Itoa(port)


	client, err := ssh.Dial("tcp", sshHost, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 3 * time.Second,
	})

	if err != nil {
		commandStr = "SSH Connect Error"
		return CommandResult{
			Target: ip,
			Cmdout: commandStr,
			Status: 1,
			Optype: opType,
		}
	}

	// 建立新会话
	session, err := client.NewSession()
	if err != nil {
		commandStr = "SSH session error"
		return CommandResult{
			Target: ip,
			Cmdout: commandStr,
			Status: 1,
			Optype: opType,
		}
	}

	// 关闭会话
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b

	if err := session.Run(command); err != nil {

		commandStr = "Failed to run: " + err.Error()
	}

	//fmt.Println(b.String())

	commandStr = b.String()

	// 关闭连接
	defer client.Close()

	return CommandResult{
		Target: ip,
		Cmdout: commandStr,
		Status: 0,
		Optype: opType,
	}

}


func RunMonitor() {

	var opType = "update"
	var cmdCommand string
	var countResult = 0

	conf := config.LoadConfig() //加载配置文件

	switch opType {

	case "scan":
		cmdCommand = "grep 'miner' 桌面/钱包配置.sh"
	case "reboot":
		cmdCommand =  monitor.RebootCommand(conf.Password)
	case "stats":
		cmdCommand =  monitor.ResponseCommand(100)
	case "update":
		cmdCommand =  monitor.UpdateConfig("39.104.86.166:3073","39.104.86.166:3072")
	default:
		cmdCommand = "grep 'miner' 桌面/钱包配置.sh"
	}

	ServerIp := MinerIPFunc(conf.IpRange)


	outchan := make(chan CommandResult)
	var wg_command sync.WaitGroup
	var wg_processing sync.WaitGroup

	// 开启执行线程
	for _, t := range ServerIp {
		wg_command.Add(1)

		target := t + " (" + conf.User + "@" + t + ")"
		go func(dst, user, ip, command string, out chan CommandResult) {
			defer wg_command.Done()
			result := remoteExec(conf.User, ip, conf.Password,opType, conf.Port, cmdCommand)
			out <- CommandResult{
				dst,
				result.Cmdout,
				result.Status,
				result.Optype,
			}
		}(target, conf.User, t, cmdCommand, outchan)
	}


	// 开启读取线程
	wg_processing.Add(1)
	go func() {
		defer wg_processing.Done()
		for o := range outchan {

			if o.Status == 0 {
				countResult += 1

				fmt.Println(o.Cmdout)
				//returnDecodeResult := monitor.DecodeMinerInfo(o.Cmdout)

				//fmt.Println("解析结果", o.Target,
				//	returnDecodeResult.Stratum, returnDecodeResult.Miner, returnDecodeResult.Worker, returnDecodeResult.PoolIP, returnDecodeResult.PoolPort)
			}
		}
	}()

	// wait untill all goroutines to finish and close the channel
	wg_command.Wait()
	close(outchan)

	wg_processing.Wait()

	fmt.Printf("miners count:%d", countResult)
}

func main() {

	RunMonitor()

}
