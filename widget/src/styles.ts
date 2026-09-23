// design token ยกมาจาก AI_OFFICE.html หัวข้อ 06
// หลักเดียว: ผู้ใช้ต้องแยกออกทันทีว่าอันไหนคำตอบของ AI อันไหนข้อมูลของระบบ
export const CSS = `
:host{
  --ground:#EDEFEE; --surface:#FAFBFA; --surface-2:#E2E7E5;
  --ink:#13282B; --ink-2:#3E5352; --muted:#778785;
  --line:#CCD5D2;
  --accent:#0F6E63; --accent-soft:#D6E7E3;
  --signal:#9A4A08; --danger:#932828; --danger-soft:#F1DBDB; --ok:#2A6D47;
  --shadow:0 1px 2px rgba(19,40,43,.06), 0 8px 24px rgba(19,40,43,.14);
  --font:"IBM Plex Sans Thai","Noto Sans Thai",system-ui,sans-serif;
  --font-head:"Anuphan","IBM Plex Sans Thai",system-ui,sans-serif;
  --font-mono:"IBM Plex Mono",ui-monospace,monospace;
}
:host([data-theme="dark"]){
  --ground:#0E1718; --surface:#152123; --surface-2:#1D2C2E;
  --ink:#E4EDEA; --ink-2:#AEC0BC; --muted:#7F918E;
  --line:#293A3C;
  --accent:#54BCAC; --accent-soft:#11302D;
  --signal:#D4913E; --danger:#DE7B7B; --danger-soft:#381D1D; --ok:#62BD89;
  --shadow:0 1px 2px rgba(0,0,0,.3), 0 8px 24px rgba(0,0,0,.35);
}
*{box-sizing:border-box}

.launcher{
  position:fixed; z-index:2147483000;
  width:56px; height:56px; border-radius:50%;
  border:0; cursor:pointer;
  background:var(--accent); color:#fff;
  box-shadow:var(--shadow);
  display:grid; place-items:center;
  font-family:var(--font-head); font-size:15px; font-weight:700;
  transition:transform .12s ease;
}
.launcher:hover{transform:translateY(-2px)}
.launcher img{width:32px;height:32px;border-radius:50%;object-fit:cover}

.panel{
  position:fixed; z-index:2147483000;
  width:380px; height:560px;
  max-width:calc(100vw - 24px); max-height:calc(100vh - 24px);
  background:var(--surface); color:var(--ink);
  border:1px solid var(--line); border-radius:16px;
  box-shadow:var(--shadow);
  display:none; flex-direction:column; overflow:hidden;
  font-family:var(--font); font-size:14.5px; line-height:1.7;
}
.panel[data-open="true"]{display:flex}

.head{
  display:flex; align-items:center; gap:10px;
  padding:12px 14px; border-bottom:1px solid var(--line);
  background:var(--surface-2);
}
.head .avatar{
  width:32px;height:32px;border-radius:50%;
  background:var(--accent); color:#fff;
  display:grid;place-items:center;
  font-family:var(--font-head);font-weight:700;font-size:13px;
  flex:0 0 auto; overflow:hidden;
}
.head .avatar img{width:100%;height:100%;object-fit:cover}
.head .name{font-family:var(--font-head);font-weight:600;font-size:15px;line-height:1.2}
/* ป้ายชื่อเว็บค้างบนหัวตลอด — ห้าม scroll หายไป */
.head .site{
  margin-left:auto; flex:0 0 auto;
  font-family:var(--font-mono); font-size:11px;
  padding:3px 9px; border-radius:99px;
  background:var(--accent-soft); color:var(--accent);
  border:1px solid var(--line);
}
.head .x{
  background:none;border:0;cursor:pointer;color:var(--muted);
  font-size:20px;line-height:1;padding:0 2px;
}

.log{flex:1;overflow-y:auto;padding:14px;display:flex;flex-direction:column;gap:10px}
.row{display:flex}
.row.me{justify-content:flex-end}
.bubble{
  max-width:84%; padding:9px 13px; border-radius:14px;
  background:var(--surface-2); border:1px solid var(--line);
  white-space:pre-wrap; word-break:break-word;
}
.row.me .bubble{background:var(--accent);color:#fff;border-color:transparent}
.note{
  font-size:12px;color:var(--muted);text-align:center;
  padding:6px 10px;font-family:var(--font-mono);
}

.foot{display:flex;gap:8px;padding:10px 12px;border-top:1px solid var(--line);background:var(--surface)}
.foot input{
  flex:1;border:1px solid var(--line);border-radius:10px;
  padding:9px 12px;font-family:var(--font);font-size:14.5px;
  background:var(--ground);color:var(--ink);
}
.foot input:focus{outline:2px solid var(--accent-soft);border-color:var(--accent)}
.foot button{
  border:0;border-radius:10px;padding:0 15px;cursor:pointer;
  background:var(--accent);color:#fff;font-family:var(--font-head);font-weight:600;
}

/* ---- โหมด preview: อยู่ในกล่อง ไม่ลอยมุมจอ ---- */
:host([data-preview="true"]) .launcher,
:host([data-preview="true"]) .panel{position:absolute}
:host([data-preview="true"]) .panel{
  inset:auto; top:12px; left:12px; right:12px; bottom:12px;
  width:auto; height:auto; display:flex;
}
`
