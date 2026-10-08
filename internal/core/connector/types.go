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

// Reindex สร้างดัชนีชื่อ tool ใหม่จาก c.Tools — ใช้กับ Connector ที่ประกอบเองในโค้ด (test) · Load ทำให้แล้ว
func (c *Connector) Reindex() {
	c.toolIndex = make(map[string]*Tool, len(c.Tools))
	for _, t := range c.Tools {
		c.toolIndex[t.Name] = t
	}
}

func (c *Connector) Tool(name string) (*Tool, bool) {
	t, ok := c.toolIndex[name]
	return t, ok
}

// PageAction หาปุ่มตาม id (host.yaml page_actions)
func (c *Connector) PageAction(id string) (PageAction, bool) {
	for _, a := range c.Host.PageActions {
		if a.ID == id {
			return a, true
		}
	}
	return PageAction{}, false
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
	Mode  string `yaml:"mode"`
	Label string `yaml:"label"`
	// Audience = ใครคุยกับผู้ช่วย: admin (ค่าเริ่มต้น · หลังบ้าน) | player (หน้าเว็บผู้เล่น) — ใช้เขียน system prompt
	Audience       string   `yaml:"audience"`
	PageAuth       PageAuth `yaml:"page_auth"`
	HostAPI        HostAPI  `yaml:"host_api"`
	Timezone       string   `yaml:"timezone"`
	DayCutoff      string   `yaml:"day_cutoff"`
	SupportMessage string   `yaml:"support_message"`
	// PageColors = ตัวแปร CSS บนหน้าเว็บที่เก็บสีของแบรนด์ — office ที่ตั้ง "ใช้สีของเว็บ" ให้ widget อ่านสีจากตรงนี้
	PageColors *PageColors `yaml:"page_colors"`
	Facts      []Fact      `yaml:"facts"`
	// PageActions = ปุ่มที่ผู้ช่วยแนบใต้คำตอบได้ (โหมด browser) · กดแล้ว widget ปิดกล่องแชทแล้วสั่งหน้าเว็บตามที่ประกาศ
	// (เปิด modal / กดปุ่มของหน้า / ไปหน้าอื่น) — โมเดลเลือกได้แค่ id · วิธีสั่งหน้าเว็บมาจากตรงนี้เท่านั้น
	PageActions []PageAction `yaml:"page_actions"`
	// Guest = ผู้ที่ยังไม่ล็อกอินคุยได้ (เฉพาะ audience: player) · ไม่ตั้ง = ต้องล็อกอินก่อนปุ่มจะขึ้น
	Guest *GuestPolicy `yaml:"guest"`
	// FollowUps = ปุ่มถามต่อตั้งต้น — ใช้เมื่อคำตอบไม่ได้ปุ่มจาก tool ข้อมูล (ทักทาย ถามเมนู ถามสถานะ ฯลฯ) · สูงสุด 3 ปุ่มต่อสถานะผู้ใช้
	FollowUps []HostFollowUp `yaml:"follow_ups"`
}

// HostFollowUp — ปุ่มถามต่อตั้งต้น · when: member | guest | any (ค่าเริ่มต้น) ตาม PageAction
type HostFollowUp struct {
	Label string `yaml:"label"`
	Ask   string `yaml:"ask"`
	When  string `yaml:"when"`
}

// AvailableTo — ปุ่มนี้ใช้ได้กับผู้ใช้สถานะนี้ไหม
func (f HostFollowUp) AvailableTo(guest bool) bool {
	return PageAction{When: f.When}.AvailableTo(guest)
}

// PageAction = ปุ่ม 1 ปุ่มใต้คำตอบ · when: member (ล็อกอินแล้ว) | guest (ยังไม่ล็อกอิน) | any (ค่าเริ่มต้น)
type PageAction struct {
	ID    string           `yaml:"id"    json:"id"`
	Label string           `yaml:"label" json:"label"`
	About string           `yaml:"about" json:"-"` // บอกโมเดลว่าปุ่มนี้พาไปไหน (ไม่ส่งลงหน้าเว็บ)
	When  string           `yaml:"when"  json:"when,omitempty"`
	Open  PageActionTarget `yaml:"open"  json:"open"`
}

// PageActionTarget — ตั้งได้อย่างเดียว:
//
//	bv_modal : id ของ modal bootstrap-vue (Nuxt 2) → window.$nuxt.$bvModal.show(id)
//	click    : CSS selector ของปุ่มที่หน้าเว็บมีอยู่แล้ว → กดให้ (ได้เงื่อนไขเดิมของหน้า เช่น ยังไม่ผูกบัญชีก็เด้งหน้าผูกบัญชี)
//	path     : หน้าในเว็บเดียวกัน (ขึ้นต้นด้วย /) → router ของหน้า หรือเปลี่ยน URL
type PageActionTarget struct {
	BVModal string `yaml:"bv_modal" json:"bv_modal,omitempty"`
	Click   string `yaml:"click"    json:"click,omitempty"`
	Path    string `yaml:"path"     json:"path,omitempty"`
}

// AvailableTo — ปุ่มนี้ใช้ได้กับผู้ใช้สถานะนี้ไหม
func (a PageAction) AvailableTo(guest bool) bool {
	switch a.When {
	case "member":
		return !guest
	case "guest":
		return guest
	}
	return true
}

// GuestPolicy — ผู้ไม่ล็อกอินถามได้วันละ DailyLimit ข้อความต่อ guest_id (นับตามวันของ timezone)
type GuestPolicy struct {
	DailyLimit int `yaml:"daily_limit"`
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
	// PermissionsToken — JWT อีกใบในหน้า (คนละใบกับที่แนบ Authorization) ที่มีสิทธิ์ติดมา · widget decode เอง
	// แล้วอ่าน Permissions.path สัมพัทธ์กับ Root ของ payload ใบนั้น · ใช้แทน PermissionsRequest (ตั้งได้อย่างใดอย่างหนึ่ง)
	PermissionsToken *TokenSource `yaml:"permissions_token" json:"permissions_token,omitempty"`

	// ---- สิทธิ์เข้า service (backend อ่านจาก JWT ที่แนบ Authorization · ไม่ส่งลงหน้าเว็บ) ----

	// Services = รายชื่อ service ที่บัญชีนี้เข้าได้ (path สัมพัทธ์กับ Root) · ไม่ตั้ง = ไม่จำกัด
	Services *PluckSpec `yaml:"services" json:"-"`
	// ServicesEmpty = รายชื่อว่างแปลว่าอะไร: all (ค่าเริ่มต้น · ไม่จำกัด) | none (เข้าไม่ได้สักเว็บ)
	ServicesEmpty string `yaml:"services_empty" json:"-"`
	// AllServicesWhen = expression ที่จริงแล้วเข้าได้ทุก service (ตัวแปร level, dept) เช่น "level >= 10 || dept == 'D002'"
	AllServicesWhen string `yaml:"all_services_when" json:"-"`
	allServicesExpr *Expr
}

// AllServices = บัญชีนี้เข้าได้ทุก service ตาม all_services_when
func (id *IdentitySpec) AllServices(level int32, dept string) bool {
	if id == nil || id.allServicesExpr == nil {
		return false
	}
	v, err := id.allServicesExpr.Eval(map[string]any{"level": float64(level), "dept": dept})
	return err == nil && v == true
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

// TokenSource — key มี {key_from} ได้ = ชื่อช่องขึ้นกับค่าในอีกช่อง
// (เช่น @nuxtjs/auth เก็บ token ที่ auth._token.<strategy> และชื่อ strategy ที่ใช้อยู่ที่ auth.strategy)
type TokenSource struct {
	Source          string      `yaml:"source"           json:"source"` // localStorage | sessionStorage
	Key             string      `yaml:"key"              json:"key"`
	KeyFrom         *KeyFromRef `yaml:"key_from"         json:"key_from,omitempty"`
	Format          string      `yaml:"format"           json:"format"` // raw | json-expiration
	ValueField      string      `yaml:"value_field"      json:"value_field,omitempty"`
	ExpirationField string      `yaml:"expiration_field" json:"expiration_field,omitempty"`
}

// ServiceSource — source: office = 1 โดเมนคือ 1 เว็บ (หน้าเว็บไม่มีตัวเลือกเว็บ เช่น หน้าผู้เล่น)
// ใช้ service แรกที่เปิดอยู่ของ office · page-config ส่งค่าไปให้ใน Value · ไม่ต้องตั้ง key
// PageColors = ชื่อตัวแปร CSS (เช่น --theme-color-1) บน :root ของหน้าเว็บ · on_accent ไม่ตั้ง = widget เลือกสีตัวอักษรเอง
type PageColors struct {
	Accent   string `yaml:"accent"    json:"accent"`
	Accent2  string `yaml:"accent_2"  json:"accent_2,omitempty"`
	OnAccent string `yaml:"on_accent" json:"on_accent,omitempty"`
}

// KeyFromRef = ช่องที่เก็บส่วนหนึ่งของชื่อช่อง token · ไม่มีค่า = ใช้ Default
type KeyFromRef struct {
	Source  string `yaml:"source"  json:"source"` // localStorage | sessionStorage
	Key     string `yaml:"key"     json:"key"`
	Default string `yaml:"default" json:"default,omitempty"`
}

type ServiceSource struct {
	Source   string `yaml:"source"   json:"source"` // localStorage | sessionStorage | query | office
	Key      string `yaml:"key"      json:"key"`
	Encoding string `yaml:"encoding" json:"encoding"` // none | base64
	Value    string `yaml:"-"        json:"value,omitempty"`
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
	// Tone = สีป้ายสถานะบนการ์ด: ok (เขียว) | wait (เหลือง) | bad (แดง) · ไม่ตั้ง = final → ok · นอกนั้น wait
	Tone string `yaml:"tone"`
}

// ToneOf — สีป้ายของสถานะนี้
func (e StatusEntry) ToneOf() string {
	if e.Tone != "" {
		return e.Tone
	}
	if e.Final {
		return "ok"
	}
	return "wait"
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
	// Keywords = คำที่บอกว่าคำถามอยู่หมวดนี้ (ทุก tool ในไฟล์) — ใช้คัด tool ก่อนส่งให้โมเดล
	Keywords []string `yaml:"keywords"`
	Tools    []*Tool  `yaml:"tools"`
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
	// FollowUps = ปุ่มถามต่อใต้คำตอบ (กดแล้วส่งเป็นคำถามใหม่) · template {input.<ชื่อ>} ได้ · สูงสุด 3
	FollowUps []FollowUp `yaml:"follow_ups"`
	// Keywords = คำที่บอกว่าคำถามเกี่ยวกับ tool นี้ (รวมกับ keywords ของไฟล์) — ใช้คัด tool ก่อนส่งให้โมเดล
	Keywords []string `yaml:"keywords"`
	// Chain = ผลของ tool นี้อาจทำให้ต้องดึงต่อ (เช่น ได้รหัสรายการ/ยูสไปค้นต่อ) — เฉพาะ tool นี้ระบบถามโมเดลว่าต้องดึงเพิ่มไหม
	Chain bool `yaml:"chain"`
	// Answer = ประโยคตอบเมื่อการ์ดตอบครบแล้ว (ไม่ต้องให้โมเดลเขียน · เร็วขึ้น 1 รอบ) · template {input.<ชื่อ>} ได้ · ไม่ตั้ง = ชื่อการ์ด
	Answer string `yaml:"answer"`

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
	Name string `yaml:"name"`
	From string `yaml:"from"` // call id
	Path string `yaml:"path"` // path ภายใน data ของ call นั้น ("" = ทั้งก้อน)
	// Paths = ลองทีละ path ใช้ตัวแรกที่มีค่า ("" = ทั้งก้อน) — ใช้แทน path เมื่อรูปข้อมูลของหลังบ้านเปลี่ยนตามการตั้งค่า
	// (เช่น /bonus ของ topupserie: data เป็นรายการ หรือ data.bonus เมื่อฝากแบบกระเป๋า)
	Paths   []string `yaml:"paths"`
	Filter  string   `yaml:"filter"` // expr ต่อ item (ตัวแปร item)
	SortBy  string   `yaml:"sort_by"`
	Order   string   `yaml:"order"` // asc | desc
	Limit   int      `yaml:"limit"`
	Agg     string   `yaml:"agg"`   // count | sum | min | max | avg | first | last
	Field   string   `yaml:"field"` // field ของ item สำหรับ agg/sort
	GroupBy string   `yaml:"group_by"`
	Expr    string   `yaml:"expr"` // คำนวณจากค่าอื่น ๆ
	Default any      `yaml:"default"`
}

type NotFoundSpec struct {
	When    string `yaml:"when"`
	Message string `yaml:"message"`
}

// FollowUp — label = ข้อความบนปุ่ม · ask = คำถามที่ส่งเมื่อกด (ไม่ตั้ง = ใช้ label)
type FollowUp struct {
	Label string `yaml:"label"`
	Ask   string `yaml:"ask"`
}

// CardStyles = แบบการ์ดที่ widget วาดได้ (ผู้ดูแลเปลี่ยนต่อ domain ในคอนโซลได้)
//
//	stat   ตัวเลขเด่น — field แรกตัวใหญ่ field ที่เหลือเป็นป้ายเล็ก (ยอดรวม/จำนวน)
//	list   รายการกะทัดรัด — แถวละรายการ (คอลัมน์ที่เป็นเงินตัวใหญ่ · สถานะเป็นป้ายสี)
//	table  ตาราง (ค่าเริ่มต้น) — สถานะเป็นป้ายสี
//	single รายการเดียว — แถวแรกของตาราง: สถานะเป็นหัว + ค่าที่เหลือเป็นช่อง 2 คอลัมน์
var CardStyles = map[string]bool{"stat": true, "list": true, "table": true, "single": true}

type CardSpec struct {
	// Style = แบบการ์ดตั้งต้นของ tool นี้ (ดู CardStyles) · ไม่ตั้ง = table
	Style  string      `yaml:"style"`
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
	When   string `yaml:"when"`  // expr: แสดงเมื่อจริง
	Empty  string `yaml:"empty"` // ค่าว่างแสดงเป็นข้อความนี้ (เช่น ผู้ทำรายการว่าง = ระบบอัตโนมัติ)
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
	Empty  string `yaml:"empty"` // ค่าว่างแสดงเป็นข้อความนี้
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

// Strings ดึงรายการสตริงจาก v ตาม spec: array ที่ Path → item ที่ WhereField == WhereValue → ค่า Pluck (ว่าง = ตัว item)
// ค่าว่าง/ไม่ใช่สตริงถูกข้าม · ตัวเลขเทียบแบบค่า (1 ใน yaml == 1.0 ใน JSON)
func (p *PluckSpec) Strings(v any) []string {
	if p == nil {
		return nil
	}
	list, ok := GetPath(v, p.Path).([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		if p.WhereField != "" && !sameValue(GetPath(item, p.WhereField), p.WhereValue) {
			continue
		}
		val := item
		if p.Pluck != "" {
			val = GetPath(item, p.Pluck)
		}
		if s, ok := val.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func sameValue(a, b any) bool {
	if fa, ok := toFloat(a); ok {
		fb, ok := toFloat(b)
		return ok && fa == fb
	}
	return a == b
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
