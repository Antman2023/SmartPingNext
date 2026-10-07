package g

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"smartping/src/internal/contextlock"
	"smartping/src/static"

	"io"
	"log"
	"maps"
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
	maxLocalConfigBytes   = 16 << 20
	configFilePermissions = 0600
	pingTargetTimeIndex   = "pinglog_target_logtime"
	alertDateIndex        = "alertlog_date"
	databaseBusyTimeoutMs = 5000
	databaseMaxOpenConns  = 16
	databaseMaxIdleConns  = 4
	databaseConnMaxIdle   = 5 * time.Minute
	cloudHTTPTimeout      = 10 * time.Second
)

var (
	Root            string
	Cfg             Config
	SelfCfg         NetworkMember
	CfgLock         sync.RWMutex
	configSaveLock  contextlock.Mutex
	AlertStatus     map[string]bool
	AlertStatusLock sync.RWMutex
	alertEpisodes   map[string]*AlertEpisode
	AuthUserIpMap   map[string]bool
	AuthAgentIpMap  map[string]bool
	AuthIpLock      sync.RWMutex
	ToolLimit       map[string]int
	ToolLimitLock   sync.RWMutex
	Db              *sql.DB
	DLock           contextlock.Mutex
	LocalTimezone   *time.Location
	HttpClient      *http.Client
)

func IsExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func ReadConfig(filename string) Config {
	config, err := readConfigFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func readConfigFile(filename string) (Config, error) {
	config := Config{}
	restrictConfigFilePermissions(filename)
	file, err := os.Open(filename)
	if err != nil {
		return config, fmt.Errorf("open config file %s: %w", filename, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxLocalConfigBytes+1))
	if err != nil {
		return config, fmt.Errorf("read config file %s: %w", filename, err)
	}
	if len(data) > maxLocalConfigBytes {
		return config, fmt.Errorf("config file %s exceeds %d bytes", filename, maxLocalConfigBytes)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("decode config file %s: %w", filename, err)
	}
	return config, nil
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
	if err := InitLogger(Root); err != nil {
		log.Fatalln("[Fault]init logger:", err)
	}
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
	if err := configureDatabasePool(Db); err != nil {
		log.Fatalln("[Fault]db pool config fail .", err)
	}
	if err := Db.Ping(); err != nil {
		log.Fatalln("[Fault]db connection fail .", err)
	}
	if err := ensureDatabaseIndexes(Db); err != nil {
		log.Fatalln("[Fault]db index migration fail .", err)
	}
	LocalTimezone = time.Local
	HttpClient = newCloudHTTPClient()
	ToolLimit = map[string]int{}
}

func newCloudHTTPClient() *http.Client {
	return &http.Client{
		Timeout: cloudHTTPTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func configureDatabasePool(db *sql.DB) error {
	if db == nil {
		return errors.New("database is nil")
	}
	db.SetMaxOpenConns(databaseMaxOpenConns)
	db.SetMaxIdleConns(databaseMaxIdleConns)
	db.SetConnMaxIdleTime(databaseConnMaxIdle)
	return nil
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
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		"CREATE INDEX IF NOT EXISTS " + pingTargetTimeIndex + " ON pinglog(target, logtime)",
		"CREATE INDEX IF NOT EXISTS " + alertDateIndex + " ON alertlog(date(logtime))",
	} {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("create database index: %w", err)
		}
	}
	return tx.Commit()
}

func SaveCloudConfig(url string) (Config, error) {
	return SaveCloudConfigContext(context.Background(), url)
}

func SaveCloudConfigContext(ctx context.Context, url string) (Config, error) {
	config := Config{}
	if err := ctx.Err(); err != nil {
		return config, err
	}
	client := HttpClient
	if client == nil {
		return config, errors.New("cloud HTTP client is not initialized")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return config, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return config, errors.New("cloud config returned non-200 status")
	}
	body, err := readCloudConfigHTTPResponseBodyContext(ctx, resp)
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
	if err := ctx.Err(); err != nil {
		return config, err
	}
	if err := applyCloudConfigContext(ctx, config, url); err != nil {
		return config, err
	}
	return config, nil
}

func readCloudConfigBody(reader io.Reader) ([]byte, error) {
	return readCloudConfigBodyContext(context.Background(), reader)
}

func readCloudConfigBodyContext(ctx context.Context, reader io.Reader) ([]byte, error) {
	return readCloudConfigBodyWithLengthContext(ctx, reader, -1)
}

func readCloudConfigBodyWithLengthContext(ctx context.Context, reader io.Reader, contentLength int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contentLength > maxCloudConfigBytes {
		return nil, errors.New("cloud config response too large")
	}
	var limited io.Reader
	if ctx.Done() != nil {
		limited = &cloudConfigContextReader{ctx: ctx, limited: io.LimitedReader{R: reader, N: maxCloudConfigBytes + 1}}
	} else {
		limited = io.LimitReader(reader, maxCloudConfigBytes+1)
	}
	var body []byte
	var err error
	if contentLength >= bytes.MinRead {
		// Reserve room for both the EOF check and the size-limit sentinel.
		buffer := bytes.NewBuffer(make([]byte, 0, int(contentLength)+bytes.MinRead+1))
		_, err = buffer.ReadFrom(limited)
		body = buffer.Bytes()
	} else {
		body, err = io.ReadAll(limited)
	}
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(body) > maxCloudConfigBytes {
		return nil, errors.New("cloud config response too large")
	}
	return body, nil
}

func readCloudConfigHTTPResponseBody(response *http.Response) ([]byte, error) {
	return readCloudConfigHTTPResponseBodyContext(context.Background(), response)
}

func readCloudConfigHTTPResponseBodyContext(ctx context.Context, response *http.Response) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("cloud config response body is missing")
	}
	return readCloudConfigBodyWithLengthContext(ctx, response.Body, response.ContentLength)
}

func ConfigSnapshot() Config {
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	return cloneConfig(Cfg)
}

func SetConfig(config Config) {
	setPreparedConfig(normalizeConfig(cloneConfig(config)))
}

// setPreparedConfig takes ownership of an already normalized, private copy.
// Callers must not mutate or retain mutable aliases into it after publication.
func setPreparedConfig(config Config) {
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

	selfConfig := cloneNetworkMember(config.Network[config.Addr])
	CfgLock.Lock()
	AuthIpLock.Lock()
	AlertStatusLock.Lock()
	nextAlertStatus := reconcileAlertStatuses(Cfg, config, AlertStatus)
	for target := range alertEpisodes {
		if healthy, exists := nextAlertStatus[target]; !exists || healthy {
			delete(alertEpisodes, target)
		}
	}
	Cfg = config
	SelfCfg = selfConfig
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

// RecordAlertCheck returns whether this result starts a new alert episode.
// Results from removed or changed rules cannot update the current state.
func RecordAlertCheck(localAddr string, rule map[string]string, healthy bool) bool {
	return RecordAlertCheckEpisode(localAddr, rule, healthy) != nil
}

// AlertEpisode identifies one transition into an unhealthy state. Its identity
// remains valid across unrelated config changes, but not recovery or replacement.
type AlertEpisode struct {
	target string
}

// RecordAlertCheckEpisode returns a retry handle only for a new alert episode.
func RecordAlertCheckEpisode(localAddr string, rule map[string]string, healthy bool) *AlertEpisode {
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	target := rule["Addr"]
	if localAddr != Cfg.Addr || target == "" || target == localAddr {
		return nil
	}
	matched := false
	for _, current := range Cfg.Network[localAddr].Topology {
		if current["Addr"] == target && ruleIdentity(current) == ruleIdentity(rule) {
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}
	// Keep the same lock order as SetConfig, holding the config read lock
	// until the state transition is published.
	AlertStatusLock.Lock()
	defer AlertStatusLock.Unlock()
	previous, exists := AlertStatus[target]
	if AlertStatus == nil {
		AlertStatus = make(map[string]bool)
	}
	AlertStatus[target] = healthy
	if healthy {
		delete(alertEpisodes, target)
		return nil
	}
	if exists && !previous {
		return nil
	}
	if alertEpisodes == nil {
		alertEpisodes = make(map[string]*AlertEpisode)
	}
	episode := &AlertEpisode{target: target}
	alertEpisodes[target] = episode
	return episode
}

// RetryAlertEpisode retries only the given episode, preserving the function API.
func RetryAlertEpisode(episode *AlertEpisode) {
	episode.Retry()
}

// Retry makes this episode eligible for another check after a canceled trace or
// failed write. A late or repeated retry cannot reset a newer alert episode.
func (episode *AlertEpisode) Retry() bool {
	if episode == nil {
		return false
	}
	AlertStatusLock.Lock()
	defer AlertStatusLock.Unlock()
	if alertEpisodes[episode.target] != episode {
		return false
	}
	delete(alertEpisodes, episode.target)
	if healthy, exists := AlertStatus[episode.target]; !exists || healthy {
		return false
	}
	AlertStatus[episode.target] = true
	return true
}

func ruleIdentity(rule map[string]string) alertRuleIdentity {
	return alertRuleIdentity{
		checkSeconds: rule["Thdchecksec"],
		loss:         rule["Thdloss"],
		averageDelay: rule["Thdavgdelay"],
		occurrences:  rule["Thdoccnum"],
	}
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
		identities[target] = ruleIdentity(rule)
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
	// Copy only the scalar under the lock; a full snapshot would clone every
	// node and topology rule on each proxy request just to read its timeout.
	CfgLock.RLock()
	v, ok := Cfg.Base[key]
	CfgLock.RUnlock()
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

func applyCloudConfigContext(ctx context.Context, downloaded Config, endpoint string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := configSaveLock.LockContext(ctx); err != nil {
		return err
	}
	defer configSaveLock.Unlock()
	// Recheck before preparing an update when cancellation races with the lock.
	if err := ctx.Err(); err != nil {
		return err
	}

	current, err := ConfigSnapshotContext(ctx)
	if err != nil {
		return err
	}
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
	unchanged := cloudConfigEqual(current, published)
	if err := ctx.Err(); err != nil {
		return err
	}
	if unchanged {
		return markCloudSyncSuccessContext(ctx, endpoint, published.Mode["LastSuccTime"])
	}
	return applyConfigLockedContext(ctx, published)
}

// The save lock protects the preceding comparison from other persisted saves.
// Refresh only runtime metadata; probe, authorization and alert state is intact.
func markCloudSyncSuccessContext(ctx context.Context, endpoint, lastSuccess string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	CfgLock.Lock()
	defer CfgLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if Cfg.Mode["Type"] != "cloud" || Cfg.Mode["Endpoint"] != endpoint {
		return errors.New("cloud configuration changed while request was in flight")
	}
	mode := cloneStringMap(Cfg.Mode)
	mode["Status"] = "true"
	mode["LastSuccTime"] = lastSuccess
	Cfg.Mode = mode
	return nil
}

func cloudConfigEqual(left, right Config) bool {
	left = cloudComparisonConfig(left)
	right = cloudComparisonConfig(right)
	for _, config := range []*Config{&left, &right} {
		delete(config.Mode, "LastSuccTime")
		delete(config.Mode, "Status")
	}
	return reflect.DeepEqual(left, right)
}

// Only copy containers changed by normalization or removal of runtime fields.
// The remaining nested data is read-only during comparison; this is not an
// independent snapshot and must not be published or mutated by the caller.
func cloudComparisonConfig(config Config) Config {
	config.Mode = maps.Clone(config.Mode)
	config.Network = maps.Clone(config.Network)
	config.Chinamap = maps.Clone(config.Chinamap)
	for province, providers := range config.Chinamap {
		// Preserve cloneConfig's normalization of nil provider maps. Address
		// slices still distinguish nil from empty, as in the original comparison.
		if providers == nil {
			config.Chinamap[province] = map[string][]string{}
		}
	}
	return normalizeConfig(config)
}

func applyConfigLocked(config Config) error {
	return applyConfigLockedContext(context.Background(), config)
}

func applyConfigLockedContext(ctx context.Context, config Config) error {
	config, err := cloneConfigContext(ctx, config)
	if err != nil {
		return err
	}
	config = normalizeConfig(config)
	if err := saveConfigFileContext(ctx, config); err != nil {
		return err
	}
	// Once persisted, finish publication even if cancellation arrives. Returning
	// early here would leave the runtime configuration behind the saved file.
	setPreparedConfig(config)
	return nil
}

func saveConfigFile(config Config) error {
	return saveConfigFileContext(context.Background(), config)
}

func saveConfigFileContext(ctx context.Context, config Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		logrus.Error("[func:SaveConfig] Json Parse ", err)
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) > maxLocalConfigBytes {
		return fmt.Errorf("serialized config exceeds %d bytes", maxLocalConfigBytes)
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
