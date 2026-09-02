package g

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxConfigTimeoutSeconds = 60
	maxConfigPort           = 65535
	maxConfigNetworkNodes   = 1024
	maxConfigTargetsPerNode = 1024
	maxConfigMappingTargets = 8192
	maxConfigAuthorizedIPs  = 4096
)

func ValidateConfig(config Config) error {
	if strings.TrimSpace(config.Name) == "" {
		return errors.New("本机节点名称为空!")
	}
	if config.Port < 1 || config.Port > maxConfigPort {
		return fmt.Errorf("非法监听端口!(1-%d)", maxConfigPort)
	}
	if !validIPv4(config.Addr) {
		return errors.New("非法本机节点IP!")
	}
	if config.Base["Timeout"] < 1 || config.Base["Timeout"] > maxConfigTimeoutSeconds {
		return fmt.Errorf("非法超时时间!(1-%d秒)", maxConfigTimeoutSeconds)
	}
	if config.Base["Archive"] < 1 || config.Base["Archive"] > 36500 {
		return errors.New("非法存档天数!(1-36500天)")
	}
	if config.Base["Refresh"] < 1 || config.Base["Refresh"] > 1440 {
		return errors.New("非法刷新频率!(1-1440分钟)")
	}
	optionalLimits := map[string][2]int{
		"PingCount":          {1, 120},
		"PingIntervalMs":     {100, 60000},
		"PingTimeoutMs":      {100, 60000},
		"PingStaggerMs":      {0, 60000},
		"MappingConcurrency": {1, 64},
		"MappingProbeCount":  {1, 20},
	}
	for key, limits := range optionalLimits {
		if value, ok := config.Base[key]; ok && (value < limits[0] || value > limits[1]) {
			return fmt.Errorf("非法配置参数 %s!(%d-%d)", key, limits[0], limits[1])
		}
	}

	if err := validatePositiveFloat(config.Topology["Tline"], 20, "非法拓扑连线粗细!(0-20)"); err != nil {
		return err
	}
	if err := validatePositiveFloat(config.Topology["Tsymbolsize"], 500, "非法拓扑形状大小!(0-500)"); err != nil {
		return err
	}
	if config.Toollimit < 0 || config.Toollimit > 86400 {
		return errors.New("非法检测工具限定频率!(0-86400秒)")
	}
	if len(config.Network) == 0 {
		return errors.New("Ping节点测试网络信息为空!")
	}
	if len(config.Network) > maxConfigNetworkNodes {
		return fmt.Errorf("Ping节点数量超过上限!(%d)", maxConfigNetworkNodes)
	}
	if _, ok := config.Network[config.Addr]; !ok {
		return errors.New("本机节点未包含在网络配置中!")
	}

	for key, member := range config.Network {
		if !validIPv4(key) || !validIPv4(member.Addr) || key != member.Addr {
			return fmt.Errorf("Ping节点测试网络信息错误!(节点键与地址不一致 %s)", key)
		}
		if strings.TrimSpace(member.Name) == "" {
			return fmt.Errorf("Ping节点测试网络信息错误!(%s 节点名称为空)", key)
		}
		if len(member.Ping) > maxConfigTargetsPerNode || len(member.Topology) > maxConfigTargetsPerNode {
			return fmt.Errorf("Ping节点测试网络信息错误!(%s 目标数量超过上限 %d)", key, maxConfigTargetsPerNode)
		}
		seenPingTargets := make(map[string]struct{}, len(member.Ping))
		for _, target := range member.Ping {
			if !validIPv4(target) {
				return fmt.Errorf("Ping节点测试网络信息错误!(非法目标IP %s)", target)
			}
			if _, ok := config.Network[target]; !ok {
				return fmt.Errorf("Ping节点测试网络信息错误!(目标节点不存在 %s)", target)
			}
			if _, exists := seenPingTargets[target]; exists {
				return fmt.Errorf("Ping节点测试网络信息错误!(重复目标节点 %s)", target)
			}
			seenPingTargets[target] = struct{}{}
		}
		seenTopologyTargets := make(map[string]struct{}, len(member.Topology))
		for _, topology := range member.Topology {
			if err := validateTopologyRule(key, topology, config.Network); err != nil {
				return err
			}
			target := topology["Addr"]
			if _, exists := seenTopologyTargets[target]; exists {
				return fmt.Errorf("Ping节点测试网络信息错误!(%s 重复拓扑目标 %s)", key, target)
			}
			seenTopologyTargets[target] = struct{}{}
		}
	}

	mappingTargets := 0
	allowedMappingCarriers := map[string]struct{}{"ctcc": {}, "cucc": {}, "cmcc": {}}
	for province, providers := range config.Chinamap {
		for carrier, ips := range providers {
			if _, ok := allowedMappingCarriers[carrier]; !ok {
				return fmt.Errorf("非法 Mapping 运营商!(%s: %s)", province, carrier)
			}
			mappingTargets += len(ips)
			if mappingTargets > maxConfigMappingTargets {
				return fmt.Errorf("Mapping IP数量超过上限!(%d)", maxConfigMappingTargets)
			}
			for _, ip := range ips {
				if ip != "" && !validIPv4(ip) {
					return errors.New("Mapping Ip illegal!")
				}
			}
		}
	}
	if err := validateAuthIPList(config.Authiplist); err != nil {
		return err
	}
	return validateMode(config.Mode)
}

func validIPv4(value string) bool {
	if value != strings.TrimSpace(value) {
		return false
	}
	ip := net.ParseIP(value)
	return ip != nil && ip.To4() != nil
}

func validatePositiveFloat(raw string, max float64, message string) error {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > max {
		return errors.New(message)
	}
	return nil
}

func validateTopologyRule(source string, rule map[string]string, network map[string]NetworkMember) error {
	target := rule["Addr"]
	if !validIPv4(target) {
		return fmt.Errorf("Ping节点测试网络信息错误!(%s 非法拓扑目标 %s)", source, target)
	}
	if _, ok := network[target]; !ok {
		return fmt.Errorf("Ping节点测试网络信息错误!(%s 拓扑目标不存在 %s)", source, target)
	}
	if strings.TrimSpace(rule["Name"]) == "" {
		return fmt.Errorf("Ping节点测试网络信息错误!(%s->%s 拓扑名称为空)", source, target)
	}
	limits := map[string][2]int{
		"Thdchecksec": {1, 86400},
		"Thdloss":     {0, 100},
		"Thdavgdelay": {1, 60000},
		"Thdoccnum":   {1, 10000},
	}
	parsedValues := make(map[string]int, len(limits))
	for key, bounds := range limits {
		value, err := strconv.Atoi(rule[key])
		if err != nil || value < bounds[0] || value > bounds[1] {
			return fmt.Errorf("Ping节点测试网络信息错误!(%s->%s 非法报警规则 %s)", source, target, key)
		}
		if key == "Thdchecksec" && value%60 != 0 {
			return fmt.Errorf("Ping节点测试网络信息错误!(%s->%s 报警检查窗口必须为60秒的倍数)", source, target)
		}
		parsedValues[key] = value
	}
	maxOccurrences := parsedValues["Thdchecksec"] / 60
	if parsedValues["Thdoccnum"] > maxOccurrences {
		return fmt.Errorf("Ping节点测试网络信息错误!(%s->%s 报警发生次数不能超过检查窗口内的样本数 %d)", source, target, maxOccurrences)
	}
	return nil
}

func validateAuthIPList(raw string) error {
	values := strings.Split(strings.ReplaceAll(raw, " ", ""), ",")
	if len(values) > maxConfigAuthorizedIPs {
		return fmt.Errorf("授权IP数量超过上限!(%d)", maxConfigAuthorizedIPs)
	}
	for _, value := range values {
		if value != "" && net.ParseIP(value) == nil {
			return fmt.Errorf("非法授权IP地址: %s", value)
		}
	}
	return nil
}

func validateMode(mode map[string]string) error {
	modeType := mode["Type"]
	if modeType == "" || modeType == "local" {
		return nil
	}
	if modeType != "cloud" {
		return errors.New("非法运行模式!")
	}
	endpoint, err := url.Parse(mode["Endpoint"])
	if err != nil || !validCloudEndpoint(endpoint) {
		return errors.New("非法云配置地址!")
	}
	return nil
}

func validCloudEndpoint(endpoint *url.URL) bool {
	if endpoint == nil ||
		!endpoint.IsAbs() ||
		(endpoint.Scheme != "http" && endpoint.Scheme != "https") ||
		endpoint.Hostname() == "" ||
		endpoint.User != nil ||
		endpoint.Fragment != "" {
		return false
	}
	port := endpoint.Port()
	if port == "" {
		return true
	}
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= maxConfigPort
}
