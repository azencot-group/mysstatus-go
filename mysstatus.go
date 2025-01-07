package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const emojiTable = "🥇🥈🥉"

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Node struct {
	// the name of the node
	Name          string
	State         string
	CfgGpuTable   map[string]int
	AllocGpuTable map[string]int
	CfgCpu        int
	AllocCpu      int
	CfgMem        string
	AllocMem      string
	Reason        string
}

const (
	IDLE      = "IDLE"
	MIXED     = "MIXED"
	ALLOCATED = "ALLOCATED"
	DRAIN     = "DRAIN"
	DOWN      = "DOWN"
	OTHER     = "OTHER"
)

func NewNode(nodeText []string) *Node {
	firstLine := nodeText[0]
	// split the first line by spaces
	nodename := strings.Split(firstLine, " ")[0][9:]
	// print the node name
	cfgLineNum := -1
	allocLineNum := -1
	sateLineNum := -1
	reasonLineNum := -1
	for i, line := range nodeText {
		if strings.Contains(line, "CfgTRES") {
			cfgLineNum = i
		}
		if strings.Contains(line, "AllocTRES") {
			allocLineNum = i
		}
		if strings.Contains(line, "State") {
			sateLineNum = i
		}
		if strings.Contains(line, "Reason") {
			reasonLineNum = i
		}
	}

	State := nodeText[sateLineNum][9:]
	for _, state := range []string{DRAIN, DOWN, IDLE, MIXED, ALLOCATED, OTHER} {
		if OTHER == state {
			State = OTHER
		} else if strings.Contains(State, state) {
			State = state
			break
		}
	}
	cfgLine := nodeText[cfgLineNum][11:]
	cpuAmountIntCfg, memAmountCfg, gpusTableCfg := getData(cfgLine)
	alloc_line := nodeText[allocLineNum][13:]
	cpuAmountIntAlloc, memAmountAlloc, gpusTableAlloc := getData(alloc_line)
	Reason := ""
	if reasonLineNum > 0 {
		Reason = nodeText[reasonLineNum][10:]
	}
	return &Node{
		Name:          nodename,
		State:         State,
		CfgGpuTable:   gpusTableCfg,
		AllocGpuTable: gpusTableAlloc,
		CfgCpu:        cpuAmountIntCfg,
		AllocCpu:      cpuAmountIntAlloc,
		CfgMem:        memAmountCfg,
		AllocMem:      memAmountAlloc,
		Reason:        Reason,
	}
}

func getData(Line string) (int, string, map[string]int) {
	Data := strings.Split(Line, ",")
	if len(Data) < 3 {
		return 0, "", make(map[string]int)
	}
	cpuAmount := strings.Split(Data[0], "=")[1]
	cpuAmountInt, err := strconv.Atoi(cpuAmount)
	if err != nil {
		panic(err)
	}
	memAmount := strings.Split(Data[1], "=")[1]
	var gpusTable = make(map[string]int)
	for _, gpu := range Data[2:] {
		if strings.Contains(gpu, "gres/gpu:") {
			gpu_name := strings.Split(strings.Replace(gpu, "gres/gpu:", "", 1), "=")[0]
			gpu_amount := strings.Split(gpu, "=")[1]
			i, err := strconv.Atoi(gpu_amount)
			if err != nil {
				panic(err)
			}
			gpusTable[gpu_name] = i
		}
	}
	return cpuAmountInt, memAmount, gpusTable
}

func printSummry(nodeList []*Node) {
	totalCfgGpu := make(map[string]int)
	totalAllocGpu := make(map[string]int)
	totalDrainGpu := make(map[string]int)
	totalDownGpu := make(map[string]int)
	drainList := make([]*Node, 0)
	downList := make([]*Node, 0)
	TotalStateTable := make(map[string]int)
	for _, node := range nodeList {
		TotalStateTable[node.State] += 1
		if node.State == DRAIN {
			drainList = append(drainList, node)
			for k, v := range node.CfgGpuTable {
				totalDrainGpu[k] += v
			}
		} else if node.State == DOWN {
			downList = append(downList, node)
			for k, v := range node.CfgGpuTable {
				totalDownGpu[k] += v
			}
		} else {
			for k, v := range node.CfgGpuTable {
				totalCfgGpu[k] += v
			}
			for k, v := range node.AllocGpuTable {
				totalAllocGpu[k] += v
			}
		}

	}
	header := "GpuType"
	underline := strings.Repeat("-", len(header))
	free := "Free   "
	used := "Used   "
	from := "From   "
	drain := "Drain  "
	down := "Down   "
	total_free := 0
	total_used := 0
	total_from := 0
	total_drain := 0
	total_down := 0
	gpuKeys := make([]string, 0)
	for k, _ := range totalCfgGpu {
		gpuKeys = append(gpuKeys, k)
	}
	sort.SliceStable(gpuKeys, func(i, j int) bool {
		return gpuKeys[i] < gpuKeys[j]
	})
	for _, k := range gpuKeys {
		header += " | " + k
		// pad free to equal the length of the header
		underline += "-|-" + strings.Repeat("-", len(k))
		space := strings.Repeat(" ", len(k)/2-1)
		free += " | " + space + strconv.Itoa(totalCfgGpu[k]-totalAllocGpu[k])
		total_free += totalCfgGpu[k] - totalAllocGpu[k]
		padLenFree := len(header) - len(free)
		padFree := strings.Repeat(" ", padLenFree)
		free += padFree
		used += " | " + space + strconv.Itoa(totalAllocGpu[k])
		total_used += totalAllocGpu[k]
		padLenUsed := len(header) - len(used)
		padUsed := strings.Repeat(" ", padLenUsed)
		used += padUsed
		from += " | " + space + strconv.Itoa(totalCfgGpu[k])
		total_from += totalCfgGpu[k]
		padLenTotal := len(header) - len(from)
		padTotal := strings.Repeat(" ", padLenTotal)
		from += padTotal
		drain += " | " + space + strconv.Itoa(totalDrainGpu[k])
		total_drain += totalDrainGpu[k]
		padLenDrain := len(header) - len(drain)
		padDrain := strings.Repeat(" ", padLenDrain)
		drain += padDrain
		down += " | " + space + strconv.Itoa(totalDownGpu[k])
		total_down += totalDownGpu[k]
		padLenDown := len(header) - len(down)
		padDown := strings.Repeat(" ", padLenDown)
		down += padDown
	}
	header += " | Total"
	underline += "-|-" + strings.Repeat("-", len("Total"))
	free += " | " + strconv.Itoa(total_free)
	used += " | " + strconv.Itoa(total_used)
	from += " | " + strconv.Itoa(total_from)
	drain += " | " + strconv.Itoa(total_drain)
	down += " | " + strconv.Itoa(total_down)
	fmt.Println(header)
	fmt.Println(underline)
	fmt.Println(free)
	fmt.Println(used)
	fmt.Println(from)
	fmt.Println(drain)
	fmt.Println(down)
	fmt.Println(underline)

	fmt.Println()
	fmt.Println("Nodes Summary:")
	totalNodes := 0
	for _, k := range []string{IDLE, MIXED, ALLOCATED, DRAIN, DOWN, OTHER} {
		v := TotalStateTable[k]
		totalNodes += v
		fmt.Println(k+":", v)
	}
	fmt.Println("Total Nodes:", totalNodes)
	fmt.Println("====================================================================================")
	fmt.Println()

	fmt.Println("Nodes Details:")
	fmt.Println("   Drain List:")
	for _, node := range drainList {
		fmt.Println("        ", node.Name, node.Reason)
	}
	fmt.Println("   Down List:")
	for _, node := range downList {
		fmt.Println("        ", node.Name, node.Reason)
	}
	fmt.Println("====================================================================================")
	fmt.Println()

}

type Job struct {
	JobID     string
	User      string
	Account   string
	Qos       string
	GpuTable  map[string]int
	CpuAmount int
	MemAmount string
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func malshine() {
	//sacctmgrByteSlice := callSccatmgr()
	//split_sacctmgr := bytes.Split(sacctmgrByteSlice, []byte("\n"))
	squeueByteSlice := callsqueue()
	split_squeue := bytes.Split(squeueByteSlice, []byte("\n"))
	UserJobTable := make(map[string][]*Job)
	userAccountTable := make(map[string]string)
	for _, line := range split_squeue[2:] {
		if len(line) > 0 {
			job := NewJob(string(line))
			UserJobTable[job.User] = append(UserJobTable[job.User], job)
			userAccountTable[job.User] = job.Account
		}
	}
	UserGpuUseTable := make(map[string]map[string]int)
	UserGpuQosTable := make(map[string]map[string]int)
	AccountGpuUseTable := make(map[string]map[string]int)
	AccountGpuQosTable := make(map[string]map[string]int)
	gpuKeys := make([]string, 0)
	for user, jobs := range UserJobTable {
		UserGpuUseTable[user] = make(map[string]int)
		UserGpuQosTable[user] = make(map[string]int)
		totalUserGpus := 0
		totalUserGpusQos := 0
		for _, job := range jobs {
			if _, ok := AccountGpuUseTable[job.Account]; !ok {
				AccountGpuUseTable[job.Account] = make(map[string]int)
				AccountGpuQosTable[job.Account] = make(map[string]int)
			}
			for k, v := range job.GpuTable {
				if stringInSlice(k, gpuKeys) == false {
					gpuKeys = append(gpuKeys, k)
				}
				UserGpuUseTable[user][k] += v
				AccountGpuUseTable[job.Account][k] += v
				if job.Qos == job.Account {
					UserGpuQosTable[user][k] += v
					AccountGpuQosTable[job.Account][k] += v
					totalUserGpusQos += v
				}
				totalUserGpus += v
			}
		}
		UserGpuUseTable[user]["total"] = totalUserGpus
		if totalUserGpusQos > 0 {
			UserGpuQosTable[user]["total"] = totalUserGpusQos
		}
	}
	sort.SliceStable(gpuKeys, func(i, j int) bool {
		return gpuKeys[i] < gpuKeys[j]
	})
	for account, _ := range AccountGpuUseTable {
		totalgpu := 0
		totalgpuqos := 0
		for k, v := range AccountGpuUseTable[account] {
			totalgpu += v
			totalgpuqos += AccountGpuQosTable[account][k]
		}
		AccountGpuUseTable[account]["total"] = totalgpu
		AccountGpuQosTable[account]["total"] = totalgpuqos
	}
	keys := make([]string, 0, len(UserGpuUseTable))
	for k := range UserGpuUseTable {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return UserGpuUseTable[keys[i]]["total"] > UserGpuUseTable[keys[j]]["total"]
	})
	fmt.Println("User Gpu Summary:")
	maxUserLen := 0
	for _, user := range keys {
		userTotalLen := len(user) + 4 + len(userAccountTable[user])
		if userTotalLen > maxUserLen {
			maxUserLen = userTotalLen
		}
	}
	padh := strings.Repeat(" ", max(maxUserLen-3, 9))
	header := "#  | User" + padh + "|  Total "
	for _, k := range gpuKeys {
		header += " | " + k
	}
	fmt.Println(header)
	underline := strings.Repeat("-", len(header))
	fmt.Println(underline)
	for i, user := range keys {
		// utf8 number
		number := strconv.Itoa(i + 1)
		if i <= 2 {
			number = emojiTable[i*4 : i*4+4]
		}
		if i < 9 && i > 2 {
			number += " "
		}
		total := strconv.Itoa(UserGpuUseTable[user]["total"]) + "(" + strconv.Itoa(UserGpuQosTable[user]["total"]) + ")"
		UserTotalLen := len(user) + 4 + len(userAccountTable[user])
		pad := strings.Repeat(" ", max(maxUserLen-UserTotalLen, 0))
		padt := strings.Repeat(" ", max(len(" Total ")-len(total), 0))
		userLine := number + " | " + user + "(" + userAccountTable[user] + ")" + pad + " | " + total + padt
		for _, k := range gpuKeys {
			numGpu := strconv.Itoa(UserGpuUseTable[user][k]) + "(" + strconv.Itoa(UserGpuQosTable[user][k]) + ")"
			padg := strings.Repeat(" ", max(len(k)-len(numGpu), 0))
			userLine += " | " + numGpu + padg
		}
		if UserGpuUseTable[user]["total"] > 0 {
			fmt.Println(userLine)
		}
	}
	fmt.Println(strings.ReplaceAll(underline, "-", "="))
	fmt.Println()
	accountKeys := make([]string, 0, len(AccountGpuUseTable))
	for k := range AccountGpuUseTable {
		accountKeys = append(accountKeys, k)
	}
	sort.SliceStable(accountKeys, func(i, j int) bool {
		return AccountGpuUseTable[accountKeys[i]]["total"] > AccountGpuUseTable[accountKeys[j]]["total"]
	})
	fmt.Println("Account Gpu Summary:")
	maxAccountLen := 0
	for _, account := range accountKeys {
		AccountTotalLen := len(account) + 2
		if AccountTotalLen > maxAccountLen {
			maxAccountLen = AccountTotalLen
		}
	}
	padh = strings.Repeat(" ", max(maxAccountLen-6, 0))
	header = "#  | Account" + padh + "|  Total "
	for _, k := range gpuKeys {
		header += " | " + k
	}
	fmt.Println(header)
	for i, account := range accountKeys {
		number := strconv.Itoa(i + 1)
		if i <= 2 {
			number = emojiTable[i*4 : i*4+4]
		}
		if i < 9 && i > 2 {
			number += " "
		}
		total := strconv.Itoa(AccountGpuUseTable[account]["total"]) + "(" + strconv.Itoa(AccountGpuQosTable[account]["total"]) + ")"
		AccountTotalLen := len(account) + 2
		pad := strings.Repeat(" ", max(maxAccountLen-AccountTotalLen, 0))
		padt := strings.Repeat(" ", max(len(" Total ")-len(total), 0))
		accountLine := number + " | " + account + pad + " | " + total + padt
		for _, k := range gpuKeys {
			numGpu := strconv.Itoa(AccountGpuUseTable[account][k]) + "(" + strconv.Itoa(AccountGpuQosTable[account][k]) + ")"
			padg := strings.Repeat(" ", max(len(k)-len(numGpu), 0))
			accountLine += " | " + numGpu + padg
		}
		if AccountGpuUseTable[account]["total"] > 0 {
			fmt.Println(accountLine)
		}
	}
	fmt.Println("====================================================================================")
	fmt.Println()
	fmt.Println("Over Usage:")
	for _, user := range keys {
		report := "User:" + user + "\n"
		toPrint := false
		for _, job := range UserJobTable[user] {
			gpuAmount := 0
			toReport := false
			jobReport := "    Job:" + job.JobID + " Account:" + job.Account + " Qos:" + job.Qos
			for _, v := range job.GpuTable {
				gpuAmount += v
			}
			if gpuAmount >= 2 {
				toReport = true
				jobReport += " Gpu:" + strconv.Itoa(gpuAmount)
			}
			if gpuAmount > 0 && job.CpuAmount > 8 {
				toReport = true
				jobReport += " Cpu:" + strconv.Itoa(job.CpuAmount)
			}
			if gpuAmount > 0 && job.MemAmount != "" {
				memAmount, err := strconv.Atoi(job.MemAmount[:len(job.MemAmount)-1])
				if err != nil {
					panic(err)
				}
				if memAmount > 100 {
					toReport = true
					jobReport += " Mem:" + job.MemAmount
				}
			}
			if toReport {
				toPrint = true
				report += jobReport + "\n"
			}
		}
		if toPrint {
			fmt.Println(report)
		}
	}
}

func NewJob(line string) *Job {
	splitLine := strings.Split(line, "|")
	// remove empty strings
	filterdSplitLine := make([]string, 0)
	for _, s := range splitLine {
		if len(s) > 0 {
			filterdSplitLine = append(filterdSplitLine, s)
		}
	}
	jobID := filterdSplitLine[0]
	User := filterdSplitLine[1]
	Account := filterdSplitLine[2]
	QOS := filterdSplitLine[4]
	Data := filterdSplitLine[len(filterdSplitLine)-1]
	DataSplit := strings.Split(Data, ",")
	gpuTable := make(map[string]int)
	cpuAmount := 0
	memAmount := ""
	for _, gpu := range DataSplit {
		if strings.Contains(gpu, "gres/gpu:") {
			gpu_name := strings.Split(strings.Replace(gpu, "gres/gpu:", "", 1), "=")[0]
			gpu_amount := strings.Split(gpu, "=")[1]
			// clean gpu_amount input before converting to int
			// replace everything that is not a number with an empty string
			gpu_amount = strings.Map(func(r rune) rune {
				if r < '0' || r > '9' {
					return -1
				}
				return r
			}, gpu_amount)
			i, err := strconv.Atoi(gpu_amount)
			if err != nil {
				panic(err)
			}
			gpuTable[gpu_name] = i
		} else if strings.Contains(gpu, "cpu=") {
			CA := strings.Split(gpu, "=")[1]
			i, err := strconv.Atoi(CA)
			if err != nil {
				panic(err)
			}
			cpuAmount = i
		} else if strings.Contains(gpu, "mem=") {
			MA := strings.Split(gpu, "=")[1]
			memAmount = MA
		}
	}
	return &Job{
		JobID:     jobID,
		User:      User,
		Account:   Account,
		Qos:       QOS,
		GpuTable:  gpuTable,
		CpuAmount: cpuAmount,
		MemAmount: memAmount,
	}
}

func callScontrol() []byte {
	cmdStruct := exec.Command("/usr/bin/scontrol", "show", "nodes", "-d")
	cmdOutput, err := cmdStruct.Output()
	if err != nil {
		panic(err)
	}
	return cmdOutput
}

func callSccat() []byte {
	cmdStruct := exec.Command("/usr/bin/sacct", "-a", "-X", "--state=running", "--format=JobId,User,Account,partition,QOS,AllocTRES%100")
	cmdOutput, err := cmdStruct.Output()
	if err != nil {
		panic(err)
	}
	return cmdOutput
}
func callSccatmgr() []byte {
	cmdStruct := exec.Command("/usr/bin/sacctmgr", "show", "account", "format=Account,Org", "-p")
	cmdOutput, err := cmdStruct.Output()
	if err != nil {
		panic(err)
	}
	return cmdOutput
}

func callsqueue() []byte {
	cmdStruct := exec.Command("/usr/bin/squeue", "--state=R", "--Format=\"JobId:|,UserName:|,Account:|,partition:|,QOS:|,tres-alloc:\"")
	cmdOutput, err := cmdStruct.Output()
	if err != nil {
		panic(err)
	}
	return cmdOutput
}

func nodeSummry() {
	byteSlice := callScontrol()
	// split on empty line
	// split the byte slice into lines
	textNodes := bytes.Split(byteSlice, []byte("\n\n"))
	// filter empty lines
	realNodes := make([][]byte, 0)
	for _, node_text := range textNodes {
		if len(node_text) > 0 {
			realNodes = append(realNodes, node_text)
		}
	}
	nodeList := make([]*Node, 0)
	for _, node := range realNodes {
		NewNode(strings.Split(string(node), "\n"))
		nodeList = append(nodeList, NewNode(strings.Split(string(node), "\n")))
	}
	printSummry(nodeList)
}

func main() {
	nodeSummry()
	malshine()
}
