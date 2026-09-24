package connector

import "sort"

// Registry = connector ทุก kind ที่โหลดตอน boot (port.ConnectorRegistry)
type Registry map[string]*Connector

func (r Registry) Get(kind string) (*Connector, bool) {
	c, ok := r[kind]
	return c, ok
}

func (r Registry) Kinds() []string {
	out := make([]string, 0, len(r))
	for k := range r {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Coverage = คำถาม BQ ที่ connector นี้ครอบ (ใช้แสดงในคอนโซล/ตรวจครบ 31 ข้อ)
func (c *Connector) Coverage() map[string][]string {
	out := map[string][]string{}
	for _, t := range c.Tools {
		for _, q := range t.Questions {
			out[q] = append(out[q], t.Name)
		}
	}
	for _, g := range c.Menus.Guides {
		for _, q := range g.Questions {
			out[q] = append(out[q], "lookup_menu:"+g.ID)
		}
	}
	for _, f := range c.Host.Facts {
		for _, q := range f.Questions {
			out[q] = append(out[q], "host_facts:"+f.ID)
		}
	}
	names := make([]string, 0, len(c.Statuses.Tables))
	for n := range c.Statuses.Tables {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, q := range c.Statuses.Tables[n].Questions {
			out[q] = append(out[q], "explain_status:"+n)
		}
	}
	return out
}
