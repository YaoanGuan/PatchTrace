package misc

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

type LocalIPChoice struct {
	Interface string
	IP        string
}

func ResolveStartupAccessIP(raw string) string {
	if ip, ok := validateStartupIP(raw); ok {
		return ip
	}
	if shouldBypassStartupIPPrompt() {
		return "127.0.0.1"
	}
	return promptStartupAccessIP()
}

func ListLocalIPChoices() ([]LocalIPChoice, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	choices := make([]LocalIPChoice, 0)
	seen := make(map[string]struct{})
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := extractAddrIP(addr)
			if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				ip = v4
			}
			ipText := ip.String()
			if _, exists := seen[ipText]; exists {
				continue
			}
			seen[ipText] = struct{}{}
			choices = append(choices, LocalIPChoice{
				Interface: iface.Name,
				IP:        ipText,
			})
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		iV4 := net.ParseIP(choices[i].IP).To4() != nil
		jV4 := net.ParseIP(choices[j].IP).To4() != nil
		if iV4 != jV4 {
			return iV4
		}
		if choices[i].Interface != choices[j].Interface {
			return choices[i].Interface < choices[j].Interface
		}
		return choices[i].IP < choices[j].IP
	})
	return choices, nil
}

func promptStartupAccessIP() string {
	choices, err := ListLocalIPChoices()
	if err != nil {
		PanicWithStack("未提供 -ip，且获取本机 IP 列表失败: %v", err)
	}

	if !isInteractiveInput() {
		printStartupIPChoices(choices)
		PanicWithStack("未提供 -ip，当前环境无法交互选择，请显式指定一个可访问 IP")
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		printStartupIPChoices(choices)
		fmt.Print("请输入编号，或直接输入自定义 IP: ")
		input, readErr := readPromptLine(reader)
		if ip, ok := selectStartupIP(reader, input, choices); ok {
			return ip
		}
		if readErr == io.EOF {
			PanicWithStack("启动已取消：未输入有效 IP")
		}
		fmt.Println("输入无效，请重新选择。")
	}
}

func printStartupIPChoices(choices []LocalIPChoice) {
	fmt.Println("请选择用于 agent_tools download_url 的可访问 IP：")
	if len(choices) == 0 {
		fmt.Println("  未发现可自动识别的本机 IP，请直接输入自定义 IP。")
		return
	}
	for i, choice := range choices {
		fmt.Printf("  %d) %s (%s)\n", i+1, choice.IP, choice.Interface)
	}
	fmt.Println("  0) 输入自定义 IP")
}

func selectStartupIP(reader *bufio.Reader, input string, choices []LocalIPChoice) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	if n, err := strconv.Atoi(input); err == nil {
		if n == 0 {
			custom := promptCustomStartupIP(reader)
			return custom, custom != ""
		}
		if n >= 1 && n <= len(choices) {
			return choices[n-1].IP, true
		}
		return "", false
	}
	return validateStartupIP(input)
}

func promptCustomStartupIP(reader *bufio.Reader) string {
	for {
		fmt.Print("请输入自定义 IP: ")
		input, readErr := readPromptLine(reader)
		if ip, ok := validateStartupIP(input); ok {
			return ip
		}
		if readErr == io.EOF {
			return ""
		}
		fmt.Println("自定义 IP 无效，请重新输入。")
	}
}

func validateStartupIP(raw string) (string, bool) {
	ip := strings.TrimSpace(raw)
	if ip == "" {
		return "", false
	}
	if net.ParseIP(ip) == nil {
		return "", false
	}
	return ip, true
}

func readPromptLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), err
}

func isInteractiveInput() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func shouldBypassStartupIPPrompt() bool {
	return false
}

func extractAddrIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}
