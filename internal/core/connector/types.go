// Package connector อ่านและตรวจ "ปลั๊กต่อชนิดหลังบ้าน" จาก connectors/<kind>/*.yaml
//
// ██ package นี้เป็นโค้ดกลาง — ห้ามมีชื่อ/พฤติกรรมของ kind ใดโดยเฉพาะ (spec §5.3 กฎปลั๊ก)
// ██ ทุกอย่างที่ต่างกันระหว่างหลังบ้าน (ชื่อช่อง localStorage, path API, สถานะ, เมนู, สิทธิ์)
// ██ ต้องอยู่ในไฟล์ yaml ของ connector เท่านั้น
package connector

// Connector = ปลั๊กของหลังบ้าน 1 ชนิด (รวมทุกไฟล์ใน connectors/<kind>/)
type Connector struct {
	Kind        string
	Dir         string
	Host        HostConfig
	Permissions Permissions
	Statuses    Statuses
	Menus       Menus
	Tools       []*Tool
	toolIndex   map[string]*Tool
}

func (c *Connector) Tool(name string) (*Tool, bool) {
	t, ok := c.toolIndex[name]
	return t, ok
}

// Rule หา permission rule ตามชื่อ (permissions.yaml)
func (c *Connector) Rule(name string) (PermissionRule, bool) {
	r, ok := c.Permissions.Rules[name]
	return r, ok
}

// ---------- host.yaml ----------

type HostConfig struct {
	Kind string `yaml:"kind"`
	// Mode = วิธีดึงข้อมูล: "host" (ค่าเริ่มต้น · backend ยิง /api/ai/read ของ host ด้วยกุญแจดอกเล็ก)
	// หรือ "browser" (widget ในหน้าแอดมินยิง API เดิมของหลังบ้านด้วย token ของแอดมินเอง — ไม่ต้องแก้หลังบ้าน)
	Mode           string   `yaml:"mode"`
	Label          string   `yaml:"label"`
	PageAuth       PageAuth `yaml:"page_auth"`
	HostAPI        HostAPI  `yaml:"host_api"`
	Timezone       string   `yaml:"timezone"`
	DayCutoff      string   `yaml:"day_cutoff"`
	SupportMessage string   `yaml:"support_message"`
	Facts          []Fact   `yaml:"facts"`
}

// HeaderSource = header ที่ widget แนบเพิ่ม (โหมด browser) เช่น header ที่หลังบ้านเดิมต้องการนอกจาก Authorization
type HeaderSource struct {
	Name   string `yaml:"name"   json:"name"`
	Source string `yaml:"source" json:"source"` // localStorage | sessionStorage | token (= token เดียวกับ Authorization)
	Key    string `yaml:"key"    json:"key,omitempty"`
	Format string `yaml:"format" json:"format,omitempty"` // raw | json-expiration (value_field ตาม token)
}

// IdentitySpec = วิธีอ่านตัวตนแอดมิน (โหมด browser)
//
//	source: jwt      → decode payload ของ token หลังบ้าน (ไม่ตรวจลายเซ็น)
//	source: request  → widget ยิง path นี้ของหลังบ้านด้วย token แอดมิน แล้วอ่านผล
//
// field เป็น path ใน object (เช่น result.username) · permissions = list ที่ pluck field ออกมา (+ where เท่ากับ)
type IdentitySpec struct {
	Source      string     `yaml:"source"       json:"source"`
	Method      string     `yaml:"method"       json:"method,omitempty"`
	Path        string     `yaml:"path"         json:"path,omitempty"` // template {service}
	Root        string     `yaml:"root"         json:"root,omitempty"` // path ของ object ผู้ใช้ใน payload/response
	ID          string     `yaml:"id"           json:"id"`
	Username    string     `yaml:"username"     json:"username"`
	DisplayName string     `yaml:"display_name" json:"display_name,omitempty"`
	Level       string     `yaml:"level"        json:"level,omitempty"`
	Dept        string     `yaml:"dept"         json:"dept,omitempty"`
	Permissions *PluckSpec `yaml:"permissions"  json:"permissions,omitempty"`
	// PermissionsRequest — path (template {service}) ของเส้นหลังบ้านที่คืนรายการสิทธิ์ของผู้ที่ล็อกอิน
	// ใช้เมื่อ JWT ไม่มีสิทธิ์ติดมา · ตั้งไว้แล้ว Permissions.path จะอ่านจาก response ของเส้นนี้แทน
	PermissionsRequest string `yaml:"permissions_request" json:"permissions_request,omitempty"`
}

type PluckSpec struct {
	Path       string `yaml:"path"        json:"path"`
	Pluck      string `yaml:"pluck"       json:"pluck,omitempty"`
	WhereField string `yaml:"where_field" json:"where_field,omitempty"`
	WhereValue any    `yaml:"where_value" json:"where_value,omitempty"`
}

// IsBrowser = connector นี้ดึงข้อมูลผ่าน browser ของแอดมิน
func (h HostConfig) IsBrowser() bool { return h.Mode == "browser" }

// PageAuth = วิธีที่ widget อ่าน "ใครล็อกอินอยู่ + เปิดเว็บไหนอยู่" จากหน้าหลังบ้าน (ส่งให้ widget ทาง page-config)
type PageAuth struct {
	Token       TokenSource   `yaml:"token"         json:"token"`
	Service     ServiceSource `yaml:"service"       json:"service"`
	HostAPIBase string        `yaml:"host_api_base" json:"host_api_base"` // template: {origin}
	SessionPath string        `yaml:"session_path"  json:"session_path"`  // template: {service}
	AuthScheme  string        `yaml:"auth_scheme"   json:"auth_scheme"`   // ค่าเริ่มต้น Bearer
	// โหมด browser: header เพิ่มที่ widget แนบตอนยิง API เดิม (ค่าอ่านจาก storage ของหน้า)
	ExtraHeaders []HeaderSource `yaml:"extra_headers" json:"extra_headers,omitempty"`
	// โหมด browser: widget หาว่าใครล็อกอินอยู่จากไหน (ไม่มีการตรวจลายเซ็น — ข้อมูลจริงยังคุมโดยหลังบ้าน)
	Identity *IdentitySpec `yaml:"identity" json:"identity,omitempty"`
}

type TokenSource struct {
	Source          string `yaml:"source"           json:"source"` // localStorage | sessionStorage
	Key             string `yaml:"key"              json:"key"`
	Format          string `yaml:"format"           json:"format"` // raw | json-expiration
	ValueField      string `yaml:"value_field"      json:"value_field,omitempty"`
	ExpirationField string `yaml:"expiration_field" json:"expiration_field,omitempty"`
}

type ServiceSource struct {
	Source   string `yaml:"source"   json:"source"` // localStorage | sessionStorage | query
	Key      string `yaml:"key"      json:"key"`
	Encoding string `yaml:"encoding" json:"encoding"` // none | base64
}

// HostAPI = วิธีที่ backend คุยกับ host แบบ server-to-server (base URL อยู่ใน DB ต่อ office)
type HostAPI struct {
	ScopedTokenPath   string   `yaml:"scoped_token_path"`
	ScopedTokenHeader string   `yaml:"scoped_token_header"`
	Envelope          Envelope `yaml:"envelope"`
	TimeoutMs         int      `yaml:"timeout_ms"`
}

// Envelope = รูปห่อ response ของ host · code ที่ไม่อยู่ใน OkCodes = error
type Envelope struct {
	CodeField    string `yaml:"code_field"`
	OkCodes      []int  `yaml:"ok_codes"`
	DataField    string `yaml:"data_field"`
	MessageField string `yaml:"message_field"`
}

// Fact = ข้อเท็จจริงคงที่ของหลังบ้านชนิดนี้ (เช่น มี/ไม่มีปุ่มฉุกเฉิน — BQ-55)
type Fact struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Text      string   `yaml:"text"`
	Keywords  []string `yaml:"keywords"`
	Questions []string `yaml:"questions"` // BQ ที่ตอบด้วย fact นี้ (รวมข้อที่ "ตอบตามจริงว่าระบบทำไม่ได้")
}

// ---------- permissions.yaml ----------

type Permissions struct {
	Rules map[string]PermissionRule `yaml:"rules"`
}

// PermissionRule ประเมินจาก permission/level ของผู้ใช้ที่ host ส่งมา (ตั๋ว) · host ตรวจซ้ำอีกชั้น
type PermissionRule struct {
	AnyOf     []string `yaml:"any_of"`
	AllOf     []string `yaml:"all_of"`
	MinLevel  int      `yaml:"min_level"`
	LoggedIn  bool     `yaml:"logged_in"`
	Menu      string   `yaml:"menu"`       // ชื่อเมนูที่ต้องมีสิทธิ์ — ใช้บอกผู้ใช้ (B-12)
	GrantHint string   `yaml:"grant_hint"` // ต้องให้ใครเปิดสิทธิ์ · ห้ามบอกวิธีข้าม
}

// Allows ตัดสินสิทธิ์ · rule ว่างทุกช่อง = ไม่ผ่าน (fail closed)
func (r PermissionRule) Allows(perms []string, level int) bool {
	has := func(code string) bool {
		for _, p := range perms {
			if p == code {
				return true
			}
		}
		return false
	}
	constrained := false
	if len(r.AnyOf) > 0 {
		constrained = true
		ok := false
		for _, c := range r.AnyOf {
			if has(c) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.AllOf) > 0 {
		constrained = true
		for _, c := range r.AllOf {
			if !has(c) {
				return false
			}
		}
	}
	if r.MinLevel > 0 {
		constrained = true
		if level < r.MinLevel {
			return false
		}
	}
	if r.LoggedIn {
		constrained = true
	}
	return constrained
}

// ---------- statuses.yaml ----------

type Statuses struct {
	UnknownMessage string                 `yaml:"unknown_message"`
	Tables         map[string]StatusTable `yaml:"tables"`
}

type StatusTable struct {
	Label     string        `yaml:"label"`
	Source    string        `yaml:"source"`    // ที่มาในโค้ดของ host (ให้คนรีวิวตามได้)
	Questions []string      `yaml:"questions"` // BQ ที่ตอบด้วยตารางนี้ (explain_status)
	Entries   []StatusEntry `yaml:"entries"`
}

type StatusEntry struct {
	Code       int      `yaml:"code"`
	Label      string   `yaml:"label"`
	Aliases    []string `yaml:"aliases"`
	Meaning    string   `yaml:"meaning"`
	NextAction string   `yaml:"next_action"`
	Final      bool     `yaml:"final"`
}

func (t StatusTable) Find(code int) (StatusEntry, bool) {
	for _, e := range t.Entries {
		if e.Code == code {
			return e, true
		}
	}
	return StatusEntry{}, false
}

// ---------- menus.yaml ----------

type Menus struct {
	Menus  []Menu  `yaml:"menus"`
	Guides []Guide `yaml:"guides"`
}

type Menu struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Path        string   `yaml:"path"`
	Group       string   `yaml:"group"`
	Permission  string   `yaml:"permission"` // ชื่อ rule ใน permissions.yaml · "" = ทุกคนที่ล็อกอิน
	Keywords    []string `yaml:"keywords"`
	Description string   `yaml:"description"`
	Tabs        []Tab    `yaml:"tabs"`
}

type Tab struct {
	Name       string `yaml:"name"`
	Permission string `yaml:"permission"`
	Note       string `yaml:"note"`
}

// Guide = วิธีทำ (ที่ AI "บอกวิธี ไม่กดให้") ดึงจากหน้าจอจริงของ host
type Guide struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Questions []string `yaml:"questions"`
	Keywords  []string `yaml:"keywords"`
	Menu      string   `yaml:"menu"`
	Tab       string   `yaml:"tab"`
	Steps     []string `yaml:"steps"`
	Fields    []string `yaml:"fields"`
	Note      string   `yaml:"note"`
}

// ---------- questions/*.yaml ----------

type QuestionFile struct {
	Questions []string `yaml:"questions"`
	Tools     []*Tool  `yaml:"tools"`
}

// Tool = ความสามารถ 1 อย่างที่โมเดลเรียกได้ (อ่านอย่างเดียว) · นิยามใน connector ไม่ใช่ในโค้ดกลาง
type Tool struct {
	Name         string               `yaml:"name"`
	Description  string               `yaml:"description"`
	Questions    []string             `yaml:"questions"`
	Permission   string               `yaml:"permission"`
	Freshness    string               `yaml:"freshness"` // live = ยอดเงินสด ห้ามแคช (B-7) · summary = แคชได้ ≤ 60 วิ
	Input        map[string]InputSpec `yaml:"input"`
	Calls        []Call               `yaml:"calls"`
	Values       []ValueSpec          `yaml:"values"`
	NotFound     *NotFoundSpec        `yaml:"not_found"`
	Card         CardSpec             `yaml:"card"`
	ModelContext []ContextSpec        `yaml:"model_context"`

	File string `yaml:"-"`
	Line int    `yaml:"-"`
}

type InputSpec struct {
	Type        string   `yaml:"type"` // string | integer | number | boolean | date
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Pattern     string   `yaml:"pattern"`
	MaxLen      int      `yaml:"max_len"`
	Enum        []any    `yaml:"enum"`
	Min         *float64 `yaml:"min"`
	Max         *float64 `yaml:"max"`
}

// Call = การยิง host 1 ครั้ง · path/query/body เป็น template
type Call struct {
	ID            string            `yaml:"id"`
	Method        string            `yaml:"method"`
	Path          string            `yaml:"path"`
	Query         map[string]string `yaml:"query"`
	Body          map[string]any    `yaml:"body"`
	TimeoutMs     int               `yaml:"timeout_ms"`
	CacheTTL      int               `yaml:"cache_ttl"`
	Envelope      string            `yaml:"envelope"` // "" = ตาม host · raw = ใช้ body ทั้งก้อน
	NotFoundCodes []int             `yaml:"not_found_codes"`
	Schema        *Schema           `yaml:"schema"`
	Optional      bool              `yaml:"optional"` // ล้มได้โดยไม่ล้มทั้ง tool
	Scope         string            `yaml:"scope"`    // โหมด browser: "" = ผูกเว็บ ({service} บังคับ) · office = ข้อมูลระดับ office
}

// ValueSpec = ค่าที่คำนวณจากผลของ call (ตามลำดับที่ประกาศ)
type ValueSpec struct {
	Name    string `yaml:"name"`
	From    string `yaml:"from"`   // call id
	Path    string `yaml:"path"`   // path ภายใน data ของ call นั้น ("" = ทั้งก้อน)
	Filter  string `yaml:"filter"` // expr ต่อ item (ตัวแปร item)
	SortBy  string `yaml:"sort_by"`
	Order   string `yaml:"order"` // asc | desc
	Limit   int    `yaml:"limit"`
	Agg     string `yaml:"agg"`   // count | sum | min | max | avg | first | last
	Field   string `yaml:"field"` // field ของ item สำหรับ agg/sort
	GroupBy string `yaml:"group_by"`
	Expr    string `yaml:"expr"` // คำนวณจากค่าอื่น ๆ
	Default any    `yaml:"default"`
}

type NotFoundSpec struct {
	When    string `yaml:"when"`
	Message string `yaml:"message"`
}

type CardSpec struct {
	Title  string      `yaml:"title"`
	Fields []FieldSpec `yaml:"fields"`
	Table  *TableSpec  `yaml:"table"`
	Note   string      `yaml:"note"`
	Link   *LinkSpec   `yaml:"link"`
}

type FieldSpec struct {
	Label  string `yaml:"label"`
	Value  string `yaml:"value"` // ชื่อ value
	Format string `yaml:"format"`
	Prefix string `yaml:"prefix"`
	Suffix string `yaml:"suffix"`
	When   string `yaml:"when"` // expr: แสดงเมื่อจริง
}

type TableSpec struct {
	Rows    string       `yaml:"rows"` // ชื่อ value ที่เป็น array
	Columns []ColumnSpec `yaml:"columns"`
	MaxRows int          `yaml:"max_rows"`
}

type ColumnSpec struct {
	Label  string `yaml:"label"`
	Field  string `yaml:"field"` // path ใน item
	Format string `yaml:"format"`
	Suffix string `yaml:"suffix"`
}

type LinkSpec struct {
	Label string `yaml:"label"`
	Path  string `yaml:"path"`
}

// ContextSpec = สิ่งเดียวที่โมเดลได้เห็นจาก tool · ห้ามเป็นตัวเลข/ชื่อ/เลขบัญชี (B-4)
//
//	type bool   : expr ที่ได้ true/false
//	type labels : pluck ค่าจาก array แล้วแปลงด้วยตารางสถานะ (ได้แต่ป้ายจากตารางของ connector)
//	type text   : ข้อความคงที่ใน connector
type ContextSpec struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Expr   string `yaml:"expr"`
	From   string `yaml:"from"`   // ชื่อ value (labels)
	Pluck  string `yaml:"pluck"`  // field ใน item (labels)
	Format string `yaml:"format"` // status:<table> (labels)
	Text   string `yaml:"text"`
}
