package g

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"smartping/src/static"

	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxCloudConfigBytes = 8 << 20

var (
	Root            string
	Cfg             Config
	SelfCfg         NetworkMember
	CfgLock         sync.RWMutex
	configSaveLock  sync.Mutex
	AlertStatus     map[string]bool
	AlertStatusLock sync.RWMutex
	AuthUserIpMap   map[string]bool
	AuthAgentIpMap  map[string]bool
	AuthIpLock      sync.RWMutex
	ToolLimit       map[string]int
	ToolLimitLock   sync.RWMutex
	Db              *sql.DB
	DLock           sync.Mutex
	LocalTimezone   *time.Location
	HttpClient      *http.Client
)

func IsExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func ReadConfig(filename string) Config {
	config := Config{}
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Config Not Found!")
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func GetRoot() string {
	//return "D:\\gopath\\src\\github.com\\smartping\\smartping"
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal("Get Root Path Error:", err)
	}
	return filepath.ToSlash(dir)
}

func releaseDefaultFiles() {
	// Create directories if not exist
	if err := os.MkdirAll(Root+"/conf", 0755); err != nil {
		log.Fatalln("[Fault]create conf directory fail:", err)
	}
	if err := os.MkdirAll(Root+"/db", 0755); err != nil {
		log.Fatalln("[Fault]create db directory fail:", err)
	}

	// Release config-base.json
	configBase := Root + "/conf/config-base.json"
	if !IsExist(configBase) {
		data, err := static.Files.ReadFile("conf/config-base.json")
		if err != nil {
			log.Fatalln("[Fault]read embedded config-base.json fail:", err)
		}
		if err := os.WriteFile(configBase, data, 0644); err != nil {
			log.Fatalln("[Fault]write config-base.json fail:", err)
		}
		log.Println("[Info]released config-base.json")
	}

	// Release database-base.db
	dbBase := Root + "/db/database-base.db"
	if !IsExist(dbBase) {
		data, err := static.Files.ReadFile("db/database-base.db")
		if err != nil {
			log.Fatalln("[Fault]read embedded database-base.db fail:", err)
		}
		if err := os.WriteFile(dbBase, data, 0644); err != nil {
			log.Fatalln("[Fault]write database-base.db fail:", err)
		}
		log.Println("[Info]released database-base.db")
	}
}

func ParseConfig(ver string) {
	Root = GetRoot()

	// Release default files if not exist
	releaseDefaultFiles()

	cfile := "config.json"
	if !IsExist(Root + "/conf/" + "config.json") {
		if !IsExist(Root + "/conf/" + "config-base.json") {
			log.Fatalln("[Fault]config file:", Root+"/conf/"+"config(-base).json", "both not existent.")
		}
		cfile = "config-base.json"
	}
	InitLogger(Root)
	config := ReadConfig(Root + "/conf/" + cfile)
	if config.Name == "" {
		config.Name, _ = os.Hostname()
	}
	if config.Addr == "" {
		config.Addr = "127.0.0.1"
	}
	config.Ver = ver
	if err := ValidateConfig(config); err != nil {
		log.Fatalln("[Fault]invalid config:", err)
	}
	SetConfig(config)
	if !IsExist(Root + "/db/" + "database.db") {
		if !IsExist(Root + "/db/" + "database-base.db") {
			log.Fatalln("[Fault]db file:", Root+"/db/"+"database(-base).db", "both not existent.")
		}
		data, err := os.ReadFile(Root + "/db/" + "database-base.db")
		if err != nil {
			log.Fatalln("[Fault]db-base file read error:", err)
		}
		if err := writeFileAtomic(Root+"/db/"+"database.db", data, 0644); err != nil {
			log.Fatalln("[Fault]db-base file copy error:", err)
		}
	}
	logrus.Info("Config loaded")
	var err error
	Db, err = sql.Open("sqlite", Root+"/db/database.db")
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
	if err := Db.Ping(); err != nil {
		log.Fatalln("[Fault]db connection fail .", err)
	}
	LocalTimezone = time.Local
	HttpClient = &http.Client{Timeout: 10 * time.Second}
	AlertStatus = map[string]bool{}
	ToolLimit = map[string]int{}
}

func SaveCloudConfig(url string) (Config, error) {
	config := Config{}
	resp, err := HttpClient.Get(url)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return config, errors.New("cloud config returned non-200 status")
	}
	body, err := readCloudConfigBody(resp.Body)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(body, &config)
	if err != nil {
		config.Name = string(body)
		return config, err
	}
	if config.Mode == nil {
		config.Mode = map[string]string{}
	}
	if err := applyCloudConfig(config, url); err != nil {
		return config, err
	}
	return config, nil
}

func readCloudConfigBody(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, maxCloudConfigBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxCloudConfigBytes {
		return nil, errors.New("cloud config response too large")
	}
	return body, nil
}

func ConfigSnapshot() Config {
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	return Cfg
}

func SetConfig(config Config) {
	config = normalizeConfig(config)
	userIPs := make(map[string]bool)
	agentIPs := make(map[string]bool)
	for _, member := range config.Network {
		agentIPs[normalizeIP(member.Addr)] = true
	}
	for _, rawIP := range strings.Split(config.Authiplist, ",") {
		if rawIP != "" {
			userIPs[rawIP] = true
		}
	}

	CfgLock.Lock()
	AuthIpLock.Lock()
	Cfg = config
	SelfCfg = config.Network[config.Addr]
	AuthUserIpMap = userIPs
	AuthAgentIpMap = agentIPs
	AuthIpLock.Unlock()
	CfgLock.Unlock()
}

func normalizeConfig(config Config) Config {
	normalizedAuthIPs := make([]string, 0)
	for _, rawIP := range strings.Split(strings.ReplaceAll(config.Authiplist, " ", ""), ",") {
		if rawIP != "" {
			normalizedAuthIPs = append(normalizedAuthIPs, normalizeIP(rawIP))
		}
	}
	config.Authiplist = strings.Join(normalizedAuthIPs, ",")
	return config
}

func SetCloudStatus(endpoint string, status string) {
	CfgLock.Lock()
	defer CfgLock.Unlock()
	if Cfg.Mode["Type"] != "cloud" || Cfg.Mode["Endpoint"] != endpoint {
		return
	}
	mode := make(map[string]string, len(Cfg.Mode)+1)
	for key, value := range Cfg.Mode {
		mode[key] = value
	}
	mode["Status"] = status
	Cfg.Mode = mode
}

func normalizeIP(value string) string {
	parsed := net.ParseIP(value)
	if parsed == nil {
		return value
	}
	if ipv4 := parsed.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return parsed.String()
}

func GetBaseInt(key string, defaultValue int) int {
	config := ConfigSnapshot()
	if config.Base == nil {
		return defaultValue
	}
	v, ok := config.Base[key]
	if !ok || v <= 0 {
		return defaultValue
	}
	return v
}

func SaveConfig() error {
	configSaveLock.Lock()
	defer configSaveLock.Unlock()
	return saveConfigFile(ConfigSnapshot())
}

func ApplyConfig(config Config) error {
	configSaveLock.Lock()
	defer configSaveLock.Unlock()
	return applyConfigLocked(config)
}

func applyCloudConfig(downloaded Config, endpoint string) error {
	configSaveLock.Lock()
	defer configSaveLock.Unlock()

	current := ConfigSnapshot()
	if current.Mode["Type"] != "cloud" || current.Mode["Endpoint"] != endpoint {
		return errors.New("cloud configuration changed while request was in flight")
	}

	published := downloaded
	published.Mode = make(map[string]string, len(downloaded.Mode)+4)
	for key, value := range downloaded.Mode {
		published.Mode[key] = value
	}
	published.Name = current.Name
	published.Addr = current.Addr
	published.Ver = current.Ver
	published.Port = current.Port
	published.Password = current.Password
	published.Mode["LastSuccTime"] = time.Now().Format("2006-01-02 15:04:05")
	published.Mode["Status"] = "true"
	published.Mode["Endpoint"] = endpoint
	published.Mode["Type"] = "cloud"
	if err := ValidateConfig(published); err != nil {
		return fmt.Errorf("invalid cloud config: %w", err)
	}
	return applyConfigLocked(published)
}

func applyConfigLocked(config Config) error {
	config = normalizeConfig(config)
	if err := saveConfigFile(config); err != nil {
		return err
	}
	SetConfig(config)
	return nil
}

func saveConfigFile(config Config) error {
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		logrus.Error("[func:SaveConfig] Json Parse ", err)
		return err
	}
	err = writeFileAtomic(filepath.Join(Root, "conf", "config.json"), data, 0644)
	if err != nil {
		logrus.Error("[func:SaveConfig] Config File Write", err)
		return err
	}
	return nil
}

func writeFileAtomic(filename string, data []byte, perm os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(filename), "."+filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(perm); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filename)
}
