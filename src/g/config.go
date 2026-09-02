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
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

const (
	maxCloudConfigBytes   = 8 << 20
	configFilePermissions = 0600
	pingTargetTimeIndex   = "pinglog_target_logtime"
	databaseBusyTimeoutMs = 5000
)

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
	restrictConfigFilePermissions(filename)
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

func restrictConfigFilePermissions(filename string) {
	if err := os.Chmod(filename, configFilePermissions); err != nil {
		log.Printf("[Warn]restrict config file permissions: %v", err)
	}
}

func GetRoot() string {
	executable, err := os.Executable()
	if err != nil {
		log.Fatal("Get Root Path Error:", err)
	}
	return filepath.ToSlash(filepath.Dir(executable))
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
		if err := os.WriteFile(configBase, data, configFilePermissions); err != nil {
			log.Fatalln("[Fault]write config-base.json fail:", err)
		}
		log.Println("[Info]released config-base.json")
	}
	restrictConfigFilePermissions(configBase)

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
	Db, err = sql.Open("sqlite", sqliteDataSource(filepath.Join(Root, "db", "database.db")))
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
	if err := Db.Ping(); err != nil {
		log.Fatalln("[Fault]db connection fail .", err)
	}
	if err := ensureDatabaseIndexes(Db); err != nil {
		log.Fatalln("[Fault]db index migration fail .", err)
	}
	LocalTimezone = time.Local
	HttpClient = &http.Client{Timeout: 10 * time.Second}
	ToolLimit = map[string]int{}
}

func sqliteDataSource(filename string) string {
	path := filepath.ToSlash(filename)
	if filepath.VolumeName(filename) != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	databaseURL := url.URL{Scheme: "file", Path: path}
	query := databaseURL.Query()
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", databaseBusyTimeoutMs))
	databaseURL.RawQuery = query.Encode()
	return databaseURL.String()
}

func ensureDatabaseIndexes(db *sql.DB) error {
	if db == nil {
		return errors.New("database is nil")
	}
	_, err := db.Exec("CREATE INDEX IF NOT EXISTS " + pingTargetTimeIndex + " ON pinglog(target, logtime)")
	return err
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
	return cloneConfig(Cfg)
}

func SetConfig(config Config) {
	config = normalizeConfig(cloneConfig(config))
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
	AlertStatusLock.Lock()
	nextAlertStatus := reconcileAlertStatuses(Cfg, config, AlertStatus)
	Cfg = config
	SelfCfg = cloneNetworkMember(config.Network[config.Addr])
	AuthUserIpMap = userIPs
	AuthAgentIpMap = agentIPs
	AlertStatus = nextAlertStatus
	AlertStatusLock.Unlock()
	AuthIpLock.Unlock()
	CfgLock.Unlock()
}

type alertRuleIdentity struct {
	checkSeconds string
	loss         string
	averageDelay string
	occurrences  string
}

func reconcileAlertStatuses(previous, next Config, current map[string]bool) map[string]bool {
	reconciled := make(map[string]bool)
	if previous.Addr == "" || previous.Addr != next.Addr {
		return reconciled
	}

	previousRules := alertRuleIdentities(previous)
	for target, identity := range alertRuleIdentities(next) {
		if previousIdentity, ok := previousRules[target]; !ok || previousIdentity != identity {
			continue
		}
		if status, ok := current[target]; ok {
			reconciled[target] = status
		}
	}
	return reconciled
}

func alertRuleIdentities(config Config) map[string]alertRuleIdentity {
	identities := make(map[string]alertRuleIdentity)
	member, ok := config.Network[config.Addr]
	if !ok {
		return identities
	}
	for _, rule := range member.Topology {
		target := rule["Addr"]
		if target == "" || target == config.Addr {
			continue
		}
		identities[target] = alertRuleIdentity{
			checkSeconds: rule["Thdchecksec"],
			loss:         rule["Thdloss"],
			averageDelay: rule["Thdavgdelay"],
			occurrences:  rule["Thdoccnum"],
		}
	}
	return identities
}

func cloneConfig(config Config) Config {
	cloned := config
	cloned.Mode = cloneStringMap(config.Mode)
	cloned.Base = cloneIntMap(config.Base)
	cloned.Topology = cloneStringMap(config.Topology)

	if config.Network != nil {
		cloned.Network = make(map[string]NetworkMember, len(config.Network))
		for key, member := range config.Network {
			cloned.Network[key] = cloneNetworkMember(member)
		}
	}

	if config.Chinamap != nil {
		cloned.Chinamap = make(map[string]map[string][]string, len(config.Chinamap))
		for carrier, provinces := range config.Chinamap {
			clonedProvinces := make(map[string][]string, len(provinces))
			for province, addresses := range provinces {
				clonedProvinces[province] = cloneStringSlice(addresses)
			}
			cloned.Chinamap[carrier] = clonedProvinces
		}
	}
	return cloned
}

func cloneNetworkMember(member NetworkMember) NetworkMember {
	cloned := member
	cloned.Ping = cloneStringSlice(member.Ping)
	if member.Topology != nil {
		cloned.Topology = make([]map[string]string, len(member.Topology))
		for i, rule := range member.Topology {
			cloned.Topology[i] = cloneStringMap(rule)
		}
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	return append(make([]string, 0, len(values)), values...)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneIntMap(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	cloned := make(map[string]int, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func normalizeConfig(config Config) Config {
	if config.Mode == nil {
		config.Mode = map[string]string{}
	}
	if config.Chinamap == nil {
		config.Chinamap = map[string]map[string][]string{}
	}
	for addr, member := range config.Network {
		if member.Ping == nil {
			member.Ping = []string{}
		}
		if member.Topology == nil {
			member.Topology = []map[string]string{}
		}
		config.Network[addr] = member
	}

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
	if cloudConfigEqual(current, published) {
		SetConfig(published)
		return nil
	}
	return applyConfigLocked(published)
}

func cloudConfigEqual(left, right Config) bool {
	left = normalizeConfig(cloneConfig(left))
	right = normalizeConfig(cloneConfig(right))
	for _, config := range []*Config{&left, &right} {
		delete(config.Mode, "LastSuccTime")
		delete(config.Mode, "Status")
	}
	return reflect.DeepEqual(left, right)
}

func applyConfigLocked(config Config) error {
	config = normalizeConfig(cloneConfig(config))
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
	err = writeFileAtomic(filepath.Join(Root, "conf", "config.json"), data, configFilePermissions)
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
