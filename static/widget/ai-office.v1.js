var AIOffice=(function(C){"use strict";var he;const me=`
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
  --accent-stops:var(--accent), var(--accent);
  --log-veil:rgba(250,251,250,.12);
  --glass:rgba(250,251,250,.78); --glass-line:rgba(255,255,255,.6);
}
:host([data-theme="dark"]){
  --ground:#0E1718; --surface:#152123; --surface-2:#1D2C2E;
  --ink:#E4EDEA; --ink-2:#AEC0BC; --muted:#7F918E;
  --line:#293A3C;
  --accent:#54BCAC; --accent-soft:#11302D;
  --signal:#D4913E; --danger:#DE7B7B; --danger-soft:#381D1D; --ok:#62BD89;
  --shadow:0 1px 2px rgba(0,0,0,.3), 0 8px 24px rgba(0,0,0,.35);
  --log-veil:rgba(14,23,24,.25);
  --glass:rgba(29,44,46,.62); --glass-line:rgba(255,255,255,.12);
}
*{box-sizing:border-box}

.launcher{
  position:fixed; z-index:2147483000;
  width:58px; height:58px; border-radius:50%; padding:0;
  border:0; cursor:pointer;
  color:var(--on-accent,#fff);
  /* แสงเงาบนสีหลัก — ให้ปุ่มดูนูน ไม่แบน (สีหลักตั้งจากคอนโซลได้) */
  background:
    radial-gradient(120% 90% at 30% 18%, rgba(255,255,255,.34), rgba(255,255,255,0) 58%),
    linear-gradient(160deg, rgba(255,255,255,0) 45%, rgba(0,0,0,.18)),
    linear-gradient(135deg, var(--accent-stops));
  box-shadow:
    inset 0 0 0 1px rgba(255,255,255,.22),
    0 2px 4px rgba(19,40,43,.12),
    0 10px 24px -6px var(--accent);
  display:grid; place-items:center;
  transition:transform .18s cubic-bezier(.2,.8,.2,1), box-shadow .18s ease;
  -webkit-tap-highlight-color:transparent;
}
.launcher:hover{
  transform:translateY(-2px) scale(1.04);
  box-shadow:inset 0 0 0 1px rgba(255,255,255,.28), 0 4px 8px rgba(19,40,43,.14), 0 16px 30px -8px var(--accent);
}
.launcher:active{transform:scale(.95)}
.launcher:focus-visible{outline:3px solid var(--accent-soft); outline-offset:3px}
/* วงแสงเบา ๆ ตอนปุ่มโผล่ครั้งแรก — บอกว่ามีผู้ช่วยแล้ว (2 รอบแล้วหยุด) */
.launcher::after{
  content:""; position:absolute; inset:0; border-radius:50%;
  box-shadow:0 0 0 0 var(--accent); opacity:.55;
  animation:aio-ping 1.8s ease-out 2; pointer-events:none;
}
@keyframes aio-ping{0%{box-shadow:0 0 0 0 var(--accent);opacity:.5}100%{box-shadow:0 0 0 16px var(--accent);opacity:0}}
.launcher .face,.launcher .close{
  grid-area:1/1; display:grid; place-items:center;
  transition:transform .22s cubic-bezier(.2,.8,.2,1), opacity .18s ease;
}
.launcher .close{opacity:0; transform:rotate(-90deg) scale(.6)}
.launcher[data-open="true"] .face{opacity:0; transform:rotate(90deg) scale(.6)}
.launcher[data-open="true"] .close{opacity:1; transform:none}
.launcher[data-open="true"]::after{animation:none; opacity:0}
.launcher img{width:46px;height:46px;border-radius:50%;object-fit:cover;box-shadow:0 0 0 2px rgba(255,255,255,.55)}
/* ปุ่มแบบฟองแชท 3D ลอย — ไม่มีวงกลมพื้นหลังตอนปิด · ตอนเปิดกลับเป็นวงกลมสีหลักพร้อม × */
.launcher[data-icon]:not([data-open="true"]){background:none;box-shadow:none;width:64px;height:64px}
.launcher[data-icon]::after{display:none}
.launcher[data-icon]:not([data-open="true"]):hover{box-shadow:none}
.licon{position:relative;display:block;width:64px;height:64px;filter:drop-shadow(0 6px 10px rgba(0,0,0,.28));animation:aio-float 3.2s ease-in-out infinite}
.licon img{width:100%;height:100%;border-radius:0;object-fit:contain;box-shadow:none}
.licon .tint{position:absolute;inset:0;background:linear-gradient(135deg, var(--accent-stops));mix-blend-mode:color;
  -webkit-mask:var(--licon) center/contain no-repeat;mask:var(--licon) center/contain no-repeat;pointer-events:none}
@keyframes aio-float{0%,100%{transform:translateY(0)}50%{transform:translateY(-3px)}}
@media (prefers-reduced-motion: reduce){.licon{animation:none}}
.ico{width:30px;height:30px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.ico .fill{fill:currentColor;stroke:none}
.launcher .close.ico{width:24px;height:24px;stroke-width:2.2}
@media (prefers-reduced-motion: reduce){
  .launcher,.launcher .face,.launcher .close{transition:none}
  .launcher::after{animation:none}
}

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
  background:var(--accent); color:var(--on-accent,#fff);
  display:grid;place-items:center;
  font-family:var(--font-head);font-weight:700;font-size:13px;
  flex:0 0 auto; overflow:hidden;
}
.head .avatar .ico{width:19px;height:19px;stroke-width:1.9}
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

.head .namewrap{min-width:0;flex:1 1 auto}
.head .tag{font-size:11.5px;line-height:1.35;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.head .tag:empty{display:none}

/* หัวแชทแบบมีรูปผู้ช่วย — ไล่สีจากสีหลัก (ตั้งรูป/คำโปรย/พื้นหลังจากคอนโซลแล้วเปิดเอง) */
.panel[data-rich] .head{
  background:
    radial-gradient(120% 140% at 0% 0%, rgba(255,255,255,.30), rgba(255,255,255,0) 60%),
    linear-gradient(120deg, rgba(255,255,255,0) 40%, rgba(0,0,0,.16)),
    linear-gradient(110deg, var(--accent-stops));
  color:var(--on-accent,#fff); border-bottom-color:transparent; padding:12px 14px;
}
.panel[data-rich] .head .avatar{width:44px;height:44px;box-shadow:0 0 0 2px rgba(255,255,255,.85), 0 2px 8px rgba(0,0,0,.18)}
.panel[data-rich] .head .name{font-size:16px}
.panel[data-rich] .head .tag{color:inherit;opacity:.9}
.panel[data-rich] .head .tag::before{content:"";display:inline-block;width:7px;height:7px;border-radius:50%;background:#4ade80;margin-right:6px;vertical-align:1px;box-shadow:0 0 0 0 rgba(74,222,128,.7);animation:aio-online 1.6s ease-out infinite}
/* จุดออนไลน์ — วงแสงขยายออกเป็นจังหวะ บอกว่าผู้ช่วยพร้อมตอบ */
@keyframes aio-online{0%{box-shadow:0 0 0 0 rgba(74,222,128,.7);opacity:1}60%{box-shadow:0 0 0 6px rgba(74,222,128,0);opacity:.75}100%{box-shadow:0 0 0 0 rgba(74,222,128,0);opacity:1}}
@media (prefers-reduced-motion: reduce){.panel[data-rich] .head .tag::before{animation:none}}
.panel[data-rich] .head .x{color:inherit;opacity:.85}
.panel[data-rich] .head .site{background:rgba(255,255,255,.18);color:inherit;border-color:rgba(255,255,255,.25)}

/* พื้นหลังห้องแชท — ลายวาดเอง (ตามสีหลัก) หรือรูป · ฟองข้อความมีพื้นของตัวเองจึงอ่านออกเสมอ */
.log[data-bg="dots"]{background-image:radial-gradient(var(--accent-soft) 1.6px, transparent 1.8px);background-size:16px 16px}
.log[data-bg="grid"]{background-image:linear-gradient(var(--accent-soft) 1px, transparent 1px),linear-gradient(90deg, var(--accent-soft) 1px, transparent 1px);background-size:22px 22px}
.log[data-bg="diagonal"]{background-image:repeating-linear-gradient(135deg, var(--accent-soft) 0 2px, transparent 2px 14px)}
.log[data-bg="glow"]{background-image:radial-gradient(90% 55% at 0% 0%, var(--accent-soft), transparent 70%),radial-gradient(90% 55% at 100% 100%, var(--accent-soft), transparent 70%)}
.log[data-bg="image"]{background:linear-gradient(var(--log-veil),var(--log-veil)),var(--log-img) center/cover no-repeat}
/* กรอบกล่องแชทไล่สีเดียวกับหัวแชท (แทนเส้นขาว/เทา) */
.panel[data-rich]{border:2px solid transparent;background:linear-gradient(var(--surface),var(--surface)) padding-box,linear-gradient(135deg, var(--accent-stops)) border-box}

/* บนรูปพื้นหลัง (โทนเข้ม) — ฟองของ AI และช่องพิมพ์เป็นกระจกเข้ม ตัวอักษรขาว ไม่ว่าธีมสว่างหรือมืด */
.panel[data-bgimg] .row.ai .bubble:not(.err){background:rgba(15,23,42,.55);color:#f1f5f9;border-color:rgba(255,255,255,.14);-webkit-backdrop-filter:blur(8px);backdrop-filter:blur(8px)}
.panel[data-bgimg] .bubble.load{color:#cbd5e1}
.panel[data-bgimg] .bubble.load .dots span{background:#cbd5e1}
.panel[data-bgimg] .sys{color:#cbd5e1;background:rgba(15,23,42,.45);border-color:rgba(255,255,255,.14)}
.panel[data-bgimg] .foot{background:rgba(10,16,30,.88);border-top-color:rgba(255,255,255,.08)}
.panel[data-bgimg] .foot textarea{background:rgba(255,255,255,.08);color:#f1f5f9;border-color:rgba(255,255,255,.16)}
.panel[data-bgimg] .foot textarea::placeholder{color:rgba(241,245,249,.55)}
.panel[data-bgimg] .foot textarea:focus{outline:2px solid rgba(255,255,255,.18);border-color:var(--accent)}

.log{flex:1;overflow-y:auto;padding:14px;display:flex;flex-direction:column;gap:10px}
.row{display:flex}
.row.me{justify-content:flex-end}
.bubble{
  max-width:88%; padding:9px 13px; border-radius:14px;
  background:var(--surface-2); border:1px solid var(--line);
  word-break:break-word;
}
.bubble .txt{white-space:pre-wrap}
.bubble .txt:empty{display:none}
.row.me .bubble{background:linear-gradient(135deg, var(--accent-stops));color:var(--on-accent,#fff);border-color:transparent}
.bubble.err{background:var(--danger-soft);border-color:transparent}
.bubble.load{color:var(--muted);font-size:13px}
.dots span{display:inline-block;width:5px;height:5px;border-radius:50%;background:var(--muted);margin-left:3px;animation:aio-bl 1.1s infinite}
.dots span:nth-child(2){animation-delay:.18s}
.dots span:nth-child(3){animation-delay:.36s}
@keyframes aio-bl{0%,80%,100%{opacity:.25}40%{opacity:1}}
@media (prefers-reduced-motion: reduce){.dots span{animation:none;opacity:.6}}
.sys{
  align-self:center;text-align:center;font-size:11.5px;color:var(--muted);
  font-family:var(--font-mono);line-height:1.6;width:100%;
  border-top:1px dashed var(--line);border-bottom:1px dashed var(--line);padding:6px 10px;
}

/* การ์ดข้อมูล = ค่าจากระบบ แยกให้เห็นชัดจากคำพูดของ AI */
.datacard{border:1px solid var(--line);border-radius:9px;background:var(--surface);padding:9px 11px;margin-top:8px;color:var(--ink)}
.datacard .dc-title{font-family:var(--font-head);font-weight:600;font-size:13px;margin-bottom:4px}
.datacard .dc-row{display:flex;justify-content:space-between;gap:12px;font-size:12.5px;padding:2px 0}
.datacard .dc-row b{font-family:var(--font-mono);font-variant-numeric:tabular-nums;font-weight:500;text-align:right}
.datacard .dc-tablewrap{overflow-x:auto;margin-top:4px}
.datacard .dc-table{border-collapse:collapse;width:100%;font-size:12px}
.datacard .dc-table th{text-align:left;color:var(--muted);font-weight:500;padding:3px 6px;border-bottom:1px solid var(--line);white-space:nowrap}
.datacard .dc-table td{padding:3px 6px;border-bottom:1px dashed var(--line);font-variant-numeric:tabular-nums;white-space:nowrap}
.datacard .dc-note{font-size:12px;color:var(--ink-2);margin-top:4px}
.datacard .src{margin-top:8px;padding-top:7px;border-top:1px dashed var(--line);display:flex;justify-content:space-between;gap:8px;font-size:11px;color:var(--muted);flex-wrap:wrap}
.datacard .src a{color:var(--accent);text-decoration:none;font-weight:500}
.datacard.k-error,.datacard.k-denied{border-color:var(--danger);background:var(--danger-soft)}
.datacard.k-not_found{border-style:dashed}

.foot{display:flex;gap:8px;padding:10px 12px;border-top:1px solid var(--line);background:var(--surface);align-items:flex-end}
.foot textarea{
  flex:1;border:1px solid var(--line);border-radius:10px;resize:none;
  padding:8px 12px;font-family:var(--font);font-size:14.5px;line-height:1.5;
  background:var(--ground);color:var(--ink);max-height:120px;min-height:38px;
}
.foot textarea:focus{outline:2px solid var(--accent-soft);border-color:var(--accent)}
.foot .send{
  border:0;border-radius:10px;padding:0 15px;height:38px;cursor:pointer;
  background:linear-gradient(135deg, var(--accent-stops));color:var(--on-accent,#fff);font-family:var(--font-head);font-weight:600;
}
.foot .send:disabled,.foot textarea:disabled{opacity:.5;cursor:default}

/* ---- จอมือถือ: เปิดเต็มจอ (ไม่ต้องแก้ธีม) · !important ทับตำแหน่งที่ applyPlacement ใส่ไว้เป็น inline ---- */
@media (max-width: 600px){
  :host(:not([data-preview="true"])) .panel{
    top:0 !important; left:0 !important; right:0 !important; bottom:0 !important;
    width:100vw; height:100vh; height:100dvh; max-width:none; max-height:none;
    border-radius:0; border:0;
    padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
  }
  /* เต็มจอแล้วปุ่มลอยบังช่องพิมพ์ — ซ่อนตอนเปิด (ปิดด้วย × บนหัวแชท) */
  :host(:not([data-preview="true"])) .launcher[data-open="true"]{display:none}
  :host(:not([data-preview="true"])) .head .x{font-size:26px;padding:4px 6px}
  /* iOS ซูมหน้าเมื่อช่องพิมพ์เล็กกว่า 16px */
  :host(:not([data-preview="true"])) .foot textarea{font-size:16px}
  .log{overscroll-behavior:contain}
}

/* ---- โหมด preview: อยู่ในกล่อง ไม่ลอยมุมจอ ---- */
:host([data-preview="true"]) .launcher,
:host([data-preview="true"]) .panel{position:absolute}
:host([data-preview="true"]) .panel{
  inset:auto; top:12px; left:12px; right:12px; bottom:12px;
  width:auto; height:auto; display:flex;
}
`;function xe(e){const t=document.createElement("div");t.setAttribute("data-ai-office-host",""),t.style.all="initial",e?(getComputedStyle(e).position==="static"&&(e.style.position="relative"),t.style.position="absolute",t.style.inset="0",e.appendChild(t)):document.body.appendChild(t);const a=t.attachShadow({mode:"closed"}),n=document.createElement("style");return n.textContent=me,a.appendChild(n),{host:t,root:a}}function ye(e){const t=document.createElement("link");t.rel="stylesheet",t.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",e.appendChild(t)}const ve=2e3;function we(e){const t=p("button","launcher");t.type="button",t.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน"),t.setAttribute("aria-expanded","false"),t.dataset.open="false";const a=p("div","panel");a.dataset.open="false",a.setAttribute("role","dialog"),a.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const n=p("div","head"),o=p("div","avatar"),r=p("div","namewrap"),i=p("div","name"),s=p("div","tag");r.append(i,s);const u=p("div","site");u.title="เว็บที่กำลังคุยอยู่";const g=p("button","x");g.type="button",g.textContent="×",g.setAttribute("aria-label","ปิด"),n.append(o,r,u,g);const c=p("div","log");c.setAttribute("aria-live","polite");const f=p("div","foot"),h=p("textarea");h.rows=1,h.maxLength=ve,h.placeholder="พิมพ์คำถาม…",h.setAttribute("aria-label","คำถาม");const d=p("button","send");d.type="button",d.textContent="↑",d.setAttribute("aria-label","ส่ง"),f.append(h,d),a.append(n,c,f),e.append(t,a);const b={launcher:t,panel:a,head:{avatar:o,name:i,tag:s,site:u},log:c,input:h,send:d};return g.addEventListener("click",()=>U(b,!1)),b}function U(e,t){e.panel.dataset.open=String(t),e.launcher.dataset.open=String(t),e.launcher.setAttribute("aria-expanded",String(t)),e.launcher.setAttribute("aria-label",t?"ปิดผู้ช่วยหลังบ้าน":"เปิดผู้ช่วยหลังบ้าน")}const Y="http://www.w3.org/2000/svg";function j(e,t){const a=document.createElementNS(Y,"svg");a.setAttribute("viewBox","0 0 24 24"),a.setAttribute("aria-hidden","true"),a.setAttribute("class",e);for(const n of t){const o=document.createElementNS(Y,"path");o.setAttribute("d",n.d),n.fill&&o.setAttribute("class","fill"),a.appendChild(o)}return a}const Z=[{d:"M3.5 6.25A3.25 3.25 0 0 1 6.75 3h10.5a3.25 3.25 0 0 1 3.25 3.25v7.5A3.25 3.25 0 0 1 17.25 17H11.5l-4.1 3.4c-.7.58-1.9.1-1.9-.83V17A3.25 3.25 0 0 1 3.5 13.75z"},{d:"M12 5.6l1.05 2.6 2.6 1.05-2.6 1.05L12 12.9l-1.05-2.6-2.6-1.05 2.6-1.05z",fill:!0}],ke=[{d:"M7 7l10 10M17 7L7 17"}];function _e(e,t){e.launcher.textContent="";const a=p("span","face");if(t){e.launcher.dataset.icon="true";const n=p("span","licon");n.style.setProperty("--licon",`url("${t}")`),n.append(te(t),p("span","tint")),a.appendChild(n)}else delete e.launcher.dataset.icon,a.appendChild(j("ico",Z));e.launcher.append(a,j("ico close",ke))}function y(e,t,a,n=""){const o=p("div","row "+t),r=p("div","bubble"+(n?" "+n:"")),i=p("div","txt");return i.textContent=a,r.appendChild(i),o.appendChild(r),e.appendChild(o),k(e),r}function Ce(e,t){const a=p("div","sys");return a.textContent=t,e.appendChild(a),k(e),a}function X(e,t){const a=p("div","row ai"),n=p("div","bubble load"),o=p("span");o.textContent=t;const r=p("span","dots");return r.append(p("span"),p("span"),p("span")),n.append(o,r),a.appendChild(n),e.appendChild(a),k(e),{set:i=>o.textContent=i,remove:()=>a.remove()}}function Q(e){var r,i,s,u,g;const t=p("div","datacard k-"+De(e.kind));if(e.title){const c=p("div","dc-title");c.textContent=e.title,t.appendChild(c)}for(const c of(r=e.fields)!=null?r:[]){const f=p("div","dc-row"),h=p("span");h.textContent=c.label;const d=p("b");d.textContent=c.display,f.append(h,d),t.appendChild(f)}if(e.table&&((i=e.table.rows)!=null&&i.length)){const c=p("div","dc-tablewrap"),f=p("table","dc-table"),h=p("thead"),d=p("tr");for(const m of(s=e.table.columns)!=null?s:[]){const x=p("th");x.textContent=m.label,d.appendChild(x)}h.appendChild(d);const b=p("tbody");for(const m of e.table.rows){const x=p("tr");for(const v of m){const $=p("td");$.textContent=(u=v==null?void 0:v.display)!=null?u:"",x.appendChild($)}b.appendChild(x)}f.append(h,b),c.appendChild(f),t.appendChild(c)}if(e.note){const c=p("div","dc-note");c.textContent=e.note,t.appendChild(c)}const a=p("div","src"),n=p("span");n.textContent=e.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+Se(e.fetched_at)+(e.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),a.appendChild(n);const o=Ae((g=e.link)==null?void 0:g.path);if(e.link&&o){const c=p("a");c.href=o,c.textContent=(e.link.label||"เปิดหน้าจริง")+" →",c.target="_self",c.rel="noopener",a.appendChild(c)}return t.appendChild(a),t}function Se(e){const t=new Date(e);if(isNaN(t.getTime()))return"-";const a={timeZone:"Asia/Bangkok"};try{const n=t.toLocaleTimeString("th-TH",{...a,hour:"2-digit",minute:"2-digit",hour12:!1}),o=t.toLocaleDateString("en-CA",a),r=new Date().toLocaleDateString("en-CA",a);if(o===r)return n;const[,i,s]=o.split("-");return`${s}/${i} ${n}`}catch{return t.toISOString().slice(0,16).replace("T"," ")}}function Ae(e){if(!e)return"";const t=e.trim();return t.startsWith("#")?t:!t.startsWith("/")||t.startsWith("//")||t.includes("\\")?"":t}function Ee(e,t){const{position:a,offset_x:n,offset_y:o}=t.placement,r=a==="bottom-left";for(const i of[e.launcher,e.panel])i.style.left="",i.style.right="";e.launcher.style.bottom=S(o),e.panel.style.bottom=S(o+68),r?(e.launcher.style.left=S(n),e.panel.style.left=S(n)):(e.launcher.style.right=S(n),e.panel.style.right=S(n))}function Te(e,t,a="",n=!0){var r,i;e.head.name.textContent=t.display_name||(n?"ผู้ช่วยหลังบ้าน":"ผู้ช่วย"),e.head.tag.textContent=(r=t.tagline)!=null?r:"",e.head.site.textContent=n&&(t.service_label||t.service_id)||"",e.head.site.style.display=e.head.site.textContent?"":"none";const o=F(t.avatar_url,a);e.head.avatar.textContent="",o?e.head.avatar.appendChild(te(o)):e.head.avatar.appendChild(j("ico",Z)),_e(e,F(t.launcher_icon,a)),o||t.tagline||t.background?e.panel.dataset.rich="true":delete e.panel.dataset.rich,$e(e.log,(i=t.background)!=null?i:"",a),e.log.dataset.bg==="image"?e.panel.dataset.bgimg="true":delete e.panel.dataset.bgimg}const Ie=/^(https:\/\/|http:\/\/localhost(:\d+)?\/)[A-Za-z0-9._~:/?#[\]@!$&*+,;=%-]+$/,Oe=new Set(["dots","grid","diagonal","glow"]);function F(e,t){if(!e)return"";if(e.startsWith("asset:")){const a=e.slice(6);return/^[a-z0-9_-]+\/[A-Za-z0-9._-]+$/.test(a)?`${t}/widget/v1/assets/${a}`:""}return Ie.test(e)?e:""}function $e(e,t,a){if(e.style.removeProperty("--log-img"),delete e.dataset.bg,t.startsWith("pattern:")){const o=t.slice(8);Oe.has(o)&&(e.dataset.bg=o);return}const n=F(t,a);n&&(e.dataset.bg="image",e.style.setProperty("--log-img",`url("${n}")`))}const ee=/^#[0-9a-f]{6}$/i;function W(e,t,a,n=""){const o=["--accent","--accent-stops","--accent-soft","--on-accent"],r=(Array.isArray(t)?t:t?[t]:[]).filter(d=>ee.test(d)).slice(0,4);if(!r.length){for(const d of o)e.style.removeProperty(d);return}const i=r[0];r.length===1&&r.push(i);const s=r.map(d=>[1,3,5].map(b=>parseInt(d.slice(b,b+2),16))),u=s[0],g=[0,1,2].map(d=>s.reduce((b,m)=>b+m[d],0)/s.length),c=a==="dark"?[21,33,35]:[255,255,255],f=d=>"#"+u.map((b,m)=>Math.round(b*d+c[m]*(1-d)).toString(16).padStart(2,"0")).join(""),h=g.map(d=>d/255).map(d=>d<=.03928?d/12.92:((d+.055)/1.055)**2.4).reduce((d,b,m)=>d+b*[.2126,.7152,.0722][m],0);e.style.setProperty("--accent",i),e.style.setProperty("--accent-stops",r.join(", ")),e.style.setProperty("--accent-soft",f(a==="dark"?.25:.16)),e.style.setProperty("--on-accent",ee.test(n)?n:h>.45?"#13282B":"#ffffff")}function Be(e){if(!e)return null;const t=getComputedStyle(document.documentElement),a=document.createElement("span");a.style.display="none",document.body.appendChild(a);const n=r=>{if(!r||!/^--[A-Za-z0-9_-]+$/.test(r))return"";const i=t.getPropertyValue(r).trim();if(!i||(a.style.color="",a.style.color=i,!a.style.color))return"";const s=/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?/.exec(getComputedStyle(a).color);return!s||s[4]!==void 0&&Number(s[4])===0?"":"#"+[s[1],s[2],s[3]].map(u=>Number(u).toString(16).padStart(2,"0")).join("")},o={accent:n(e.accent),accent2:n(e.accent_2),on:n(e.on_accent)};return a.remove(),o.accent?o:null}function k(e){e.scrollTop=e.scrollHeight}function te(e){const t=document.createElement("img");return t.src=e,t.alt="",t.referrerPolicy="no-referrer",t}function De(e){return String(e||"").replace(/[^a-z_]/g,"")}function S(e){return`${e|0}px`}function p(e,t){const a=document.createElement(e);return t&&(a.className=t),a}const Pe=["avatar_url","display_name","greeting","tagline","launcher_icon","background","theme","accent_color","accent_colors","color_source","placement","service_label","is_hidden"],ze=2*1024*1024;function Le(e,t,a){if(!e||typeof t!="string"||!t.startsWith("/")||t.startsWith("//")||t.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(t)||t.includes("://"))return null;let n;try{n=new URL(e)}catch{return null}const o=n.pathname.replace(/\/+$/,"");let r;try{r=new URL(n.origin+o+t)}catch{return null}if(r.origin!==n.origin||!(r.pathname===o||r.pathname.startsWith(o+"/")))return null;if(a)for(const[i,s]of Object.entries(a))r.searchParams.set(i,String(s));return r.toString()}async function ae(e,t,a,n="Bearer"){var u;if(!a)return{status:401,body:""};const o=String(t.method||"").toUpperCase();if(o!=="GET"&&o!=="POST")return{status:0,body:""};const r=Le(e,t.path,t.query);if(!r)return{status:0,body:""};const i=new AbortController,s=setTimeout(()=>i.abort(),Math.max(1e3,(u=t.timeout_ms)!=null?u:2e4));try{const g={method:o,credentials:"omit",signal:i.signal,headers:{Authorization:`${n} ${a}`,Accept:"application/json"}};o==="POST"&&t.body!==void 0&&t.body!==null&&(g.headers["Content-Type"]="application/json",g.body=JSON.stringify(t.body));const c=await fetch(r,g),f=await c.text();return f.length>ze?{status:0,body:""}:{status:c.status,body:f}}catch{return{status:0,body:""}}finally{clearTimeout(s)}}const A=new Set(["","null","undefined","false"]);function H(e){try{if(e==="localStorage")return localStorage;if(e==="sessionStorage")return sessionStorage}catch{}return null}function ne(e){var t,a;if(!e)return"";try{const n=(a=(t=H(e.source))==null?void 0:t.getItem(Ne(e)))!=null?a:"";if(A.has(n.trim()))return"";if(e.format!=="json-expiration")return oe(n);const o=JSON.parse(n),r=o==null?void 0:o[e.value_field||"value"];if(typeof r!="string"||A.has(r))return"";const i=o[e.expiration_field||"expiration"];return typeof i=="number"&&i<=Math.floor(Date.now()/1e3)?"":oe(r)}catch{return""}}function Ne(e){var a,n,o;if(!e.key_from)return e.key;const t=((n=(a=H(e.key_from.source))==null?void 0:a.getItem(e.key_from.key))!=null?n:"").trim();return e.key.split("{key_from}").join(A.has(t)?(o=e.key_from.default)!=null?o:"":t)}function oe(e){const t=e.trim(),a=/^bearer(\s+|$)/i.exec(t),n=a?t.slice(a[0].length).trim():t;return A.has(n)?"":n}function G(e){var t,a,n,o,r;if(!e)return"";if(e.source==="office")return(t=e.value)!=null?t:"";try{let i="";if(e.source==="query"){if(i=(a=new URLSearchParams(location.search).get(e.key))!=null?a:"",!i){const s=location.hash.indexOf("?");s>=0&&(i=(n=new URLSearchParams(location.hash.slice(s+1)).get(e.key))!=null?n:"")}}else i=(r=(o=H(e.source))==null?void 0:o.getItem(e.key))!=null?r:"";if(i=i.trim(),A.has(i))return"";if(e.encoding==="base64"){const s=atob(i).trim();return A.has(s)?"":s}return i}catch{return""}}function re(e){return e?e.source==="office"?"service ของ domain นี้ (เปิดอย่างน้อย 1 service ที่คอนโซล)":e.source==="query"?`?${e.key}=`:`${e.source}["${e.key}"]`:"(ไม่ทราบ)"}function Re(e){const t=e.split(".")[1];if(!t)return null;try{const a=t.replace(/-/g,"+").replace(/_/g,"/"),n=atob(a+"=".repeat((4-a.length%4)%4)),o=Uint8Array.from(n,r=>r.charCodeAt(0));return JSON.parse(new TextDecoder().decode(o))}catch{return null}}function ie(e,t){const a=B(e,t.path);if(!Array.isArray(a))return[];const n=[];for(const o of a){if(t.where_field&&B(o,t.where_field)!==t.where_value)continue;const r=t.pluck?B(o,t.pluck):o;typeof r=="string"&&r&&n.push(r)}return n}function B(e,t){let a=e;if(!t)return a;for(const n of t.split(".")){if(!a||typeof a!="object")return;a=a[n]}return a}const Me=180*1e3;class Ue{constructor(t,a=""){this.apiBase=t,this.hostAPIBase=a,this.cfg=null,this.loading=null,this.cur=null}hostApiBase(){var t;return this.hostAPIBase||((t=this.cfg)==null?void 0:t.host_api_base)||""}readToken(){var t;return ne((t=this.cfg)==null?void 0:t.token)}authScheme(){var t;return((t=this.cfg)==null?void 0:t.auth_scheme)||"Bearer"}async pageConfig(){var t;return this.cfg?this.cfg:((t=this.loading)!=null||(this.loading=(async()=>{var o;const a=await fetch(`${this.apiBase}/api/ai/widget/page-config`),n=await a.json().catch(()=>null);if(!a.ok||!(n!=null&&n.payload))throw new se((o=n==null?void 0:n.message)!=null?o:String(a.status));return this.cfg=n.payload,this.cfg})().finally(()=>{this.loading=null})),this.loading)}async ticket(t){var i;if(this.cur&&this.cur.service===t&&this.cur.expiresAt-Date.now()>Me)return this.cur.ticket;const a=await this.pageConfig(),n=await this.readPermissions(a,t),o=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(t)}/browser-session`,{method:"POST",headers:{Authorization:`Bearer ${this.readToken()}`,"Content-Type":"application/json"},body:JSON.stringify({permissions:n})}),r=await o.json().catch(()=>null);if(!o.ok||!((i=r==null?void 0:r.payload)!=null&&i.ticket))throw new Error((r==null?void 0:r.error)||`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${o.status})`);return this.cur={service:t,ticket:r.payload.ticket,expiresAt:Date.now()+r.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(t,a){const n=t.identity,o=n==null?void 0:n.permissions;if(!(o!=null&&o.path))return[];if(n!=null&&n.permissions_token){const s=ne(n.permissions_token);return s?ie(B(Re(s),n.root),o):[]}const r=n==null?void 0:n.permissions_request;if(!r)return[];const i=await ae(this.hostApiBase(),{id:"perm",method:"GET",path:r.split("{service}").join(encodeURIComponent(a))},this.readToken(),this.authScheme());if(i.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${i.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{return ie(JSON.parse(i.body),o)}catch{return[]}}}class se extends Error{}class E extends Error{}async function je(e){const{apiBase:t,service:a,session:n,on:o}=e,r=await n.ticket(a),i=`${t}/api/ai/widget/service/${encodeURIComponent(a)}`;let s=e.conversationID;const u=await fetch(`${i}/chat`,{method:"POST",headers:{Authorization:`Bearer ${r}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:s,text:e.text})});if(!u.ok||!u.body){u.status===401&&n.invalidate();const h=await u.json().catch(()=>null);throw new E((h==null?void 0:h.error)||`เซิร์ฟเวอร์ตอบ ${u.status}`)}const g=[],c=h=>g.push(ae(n.hostApiBase(),h,n.readToken(),n.authScheme()).then(d=>fetch(`${i}/chat/relay/${encodeURIComponent(h.id)}`,{method:"POST",headers:{Authorization:`Bearer ${r}`,"Content-Type":"application/json"},body:JSON.stringify(d)})).then(()=>{}).catch(()=>{}));let f=null;if(await ce(u.body,(h,d)=>{var m,x,v;const b=d;switch(h){case"status":typeof b.conversation_id=="string"&&(s=b.conversation_id),o.status(String((m=b.text)!=null?m:""));break;case"fetch":c(b);break;case"card":o.card(b);break;case"token":o.token(String((x=b.text)!=null?x:""));break;case"error":f=new E(String((v=b.message)!=null?v:"ผู้ช่วยตอบไม่สำเร็จ"));break}}),await Promise.all(g),f)throw f;return s}async function Fe(e){const t=await fetch(`${e.apiBase}/api/ai/admin/assistant`,{method:"POST",headers:{Authorization:`Bearer ${e.token}`,"Content-Type":"application/json"},body:JSON.stringify({messages:e.messages,page:e.page})});if(!t.ok||!t.body){const n=await t.json().catch(()=>null);throw new E((n==null?void 0:n.error)||`เซิร์ฟเวอร์ตอบ ${t.status}`)}let a=null;if(await ce(t.body,(n,o)=>{var i,s,u;const r=o;n==="status"?e.on.status(String((i=r.text)!=null?i:"")):n==="card"?e.on.card(r):n==="token"?e.on.token(String((s=r.text)!=null?s:"")):n==="error"&&(a=new E(String((u=r.message)!=null?u:"ผู้ช่วยตอบไม่สำเร็จ")))}),a)throw a}async function ce(e,t){const a=e.getReader(),n=new TextDecoder;let o="";for(;;){const{value:r,done:i}=await a.read();if(i)break;o+=n.decode(r,{stream:!0});let s;for(;(s=o.indexOf(`

`))>=0;){const u=o.slice(0,s);o=o.slice(s+2);let g="message",c="";for(const f of u.split(`
`))f.startsWith("event:")?g=f.slice(6).trim():f.startsWith("data:")&&(c+=f.slice(5).trim());c&&t(g,JSON.parse(c))}}}const We=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],J=new Map;function He(){J.clear()}function le(e,t=""){const a=e+"|"+t;let n=J.get(a);return n||(n=new Ue(e,t),J.set(a,n)),n}const D={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}},Ge=20;let l=null;function P(){window.removeEventListener("message",pe),N(),l==null||l.host.remove(),l=null}async function z(e={}){var h,d,b,m,x,v,$,ge,be;P();const t=(h=e.dataset)!=null?h:{};for(const w of We)t[w]&&(console.warn(`[ai-office] ไม่รับ data-${Ze(w)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete t[w]);const a=(d=t.previewMount)!=null?d:"",n=a!=="",o=(m=(b=e.apiBase)!=null?b:t.apiBase)!=null?m:"",r=le(o,t.hostApiBase);let i;if(e.console)i={...D,display_name:(x=e.console.displayName)!=null?x:"ผู้ช่วย AI Office",greeting:(v=e.console.greeting)!=null?v:"สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)",service_label:($=e.console.label)!=null?$:"คอนโซล",placement:{position:"bottom-right",offset_x:20,offset_y:20}};else if(n)i={...D,...(ge=e.bootstrap)!=null?ge:{}};else if(e.bootstrap)i={...D,...e.bootstrap};else{const w=await Je(o,r,e.onFetch);if(!w)return;i={...D,...w}}if(!n&&!i.enabled)return;const s=n?document.querySelector(a):null;if(n&&!s){console.warn(`[ai-office] ไม่พบกล่อง preview: ${a}`);return}const{host:u,root:g}=xe(s);n&&u.setAttribute("data-preview","true"),ye(g);const c=we(g),f=!n&&!e.console&&!e.bootstrap?await r.pageConfig().catch(()=>null):null;l={ui:c,host:u,cfg:i,preview:n,apiBase:o,session:r,conversationID:"",busy:!1,console:(be=e.console)!=null?be:null,history:[],showSite:(f==null?void 0:f.audience)!=="player",pageColors:f==null?void 0:f.page_colors},de(i),c.launcher.addEventListener("click",()=>I()),c.send.addEventListener("click",()=>R(e)),c.input.addEventListener("keydown",w=>{const M=w;M.key==="Enter"&&!M.shiftKey&&!M.isComposing&&(M.preventDefault(),R(e))}),n&&(U(c,!0),window.addEventListener("message",pe)),e.console||Xe()}function de(e){if(!l)return;l.cfg=e;const t=ue(e.theme);l.host.setAttribute("data-theme",t);const a=K(t);Te(l.ui,e,l.apiBase,l.showSite),Ee(l.ui,e),l.ui.launcher.style.display=e.is_hidden?"none":"grid",l.ui.log.textContent="",e.greeting&&y(l.ui.log,"ai",e.greeting),l.preview&&Ce(l.ui.log,_.preview),a||Ke()}function pe(e){if(e.origin!==window.location.origin)return;const t=e.data;if(!t||t.type!=="ai-office:preview-config"||!t.config||!l||!l.preview)return;const a={};for(const n of Pe)n in t.config&&(a[n]=t.config[n]);de({...l.cfg,...a})}async function Je(e,t,a){var s,u,g,c,f;const n=await q(t);if(!n)return null;const o=t.readToken(),r=G(n.service);if(!o||!r){const h=re(n.token),d=re(n.service);return T(!o&&!r?`ไม่พบ ${h} และ ${d} — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน`:o?`ไม่พบ ${d} — ยังไม่ได้เลือกเว็บในหลังบ้าน (หน้านี้ไม่ได้ผูกกับเว็บใด)`:`ไม่พบ ${h} ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว`),null}const i=`${e}/api/ai/widget/service/${encodeURIComponent(r)}/bootstrap`;a==null||a({url:i});try{const h=await fetch(i,{headers:{Authorization:`Bearer ${o}`}}),d=await h.json().catch(()=>null);if(!h.ok)return T(`เซิร์ฟเวอร์ตอบ ${h.status} ${(s=d==null?void 0:d.message)!=null?s:""} — ${(u=d==null?void 0:d.error)!=null?u:""}`.trim(),V[(g=d==null?void 0:d.message)!=null?g:""]),null;const b=(c=d==null?void 0:d.payload)!=null?c:null;return b&&!b.enabled&&T(`ยังไม่เปิดใช้งาน (reason: ${b.reason})`,V[(f=b.reason)!=null?f:""]),b}catch(h){return T(`เรียก ${i} ไม่สำเร็จ — ${h.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}async function q(e){var t;try{return await e.pageConfig()}catch(a){const n=a instanceof se?a.message:"";return T(`โหลดการตั้งค่าของหน้านี้ไม่สำเร็จ (${n||a.message})`,(t=V[n])!=null?t:"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function T(e,t){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${e}`+(t?`
           → ${t}`:""))}const V={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์เปิดเว็บนี้ในหลังบ้าน (รายชื่อเว็บของบัญชี)",KIND_NOT_SUPPORTED:'ชนิดหลังบ้านของ domain นี้ยังไม่มี connector — ตรวจ "ชนิดหลังบ้าน" ที่คอนโซล',SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function K(e=ue((t=>(t=l==null?void 0:l.cfg.theme)!=null?t:"auto")())){var o;if(!l)return!0;const a=l.cfg,n=a.color_source==="site"?Be(l.pageColors):null;return n?W(l.host,[n.accent,n.accent2||n.accent],e,n.on):a.color_source==="site"?W(l.host,void 0,e):W(l.host,(o=a.accent_colors)!=null&&o.length?a.accent_colors:a.accent_color,e),a.color_source!=="site"||!!n}const qe=4e3,Ve=2e4;let L=null;function Ke(){N();const e=l;if(!e||e.preview)return;const t=Date.now();e.ui.launcher.style.visibility="hidden",L=setInterval(()=>{if(l!==e)return N();const a=K(),n=Date.now()-t;(a||n>qe)&&(e.ui.launcher.style.visibility=""),(a||n>Ve)&&N()},250)}function N(){L&&clearInterval(L),L=null}function I(e){if(!l)return;const t=e!=null?e:l.ui.panel.dataset.open!=="true";t&&K(),U(l.ui,t),t&&l.ui.input.focus()}const _={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)"};async function R(e){var g;const t=l;if(!t||t.busy||t.ui.input.disabled)return;const a=t.ui.input.value.trim();if(!a)return;if(t.ui.input.value="",y(t.ui.log,"me",a),t.preview){y(t.ui.log,"ai",_.preview);return}if(t.console){await Ye(t,t.console,a);return}const n=await q(t.session),o=n?G(n.service):"";if(!o){y(t.ui.log,"ai",_.unavailable,"err");return}(g=e.onFetch)==null||g.call(e,{url:`${t.apiBase}/api/ai/widget/service/${encodeURIComponent(o)}/chat`,body:{text:a}}),t.busy=!0,t.ui.send.disabled=!0;const r=X(t.ui.log,"กำลังส่งคำถาม…");let i=null,s=null;const u=()=>{if(i)return;const c=y(t.ui.log,"ai","");i=c.querySelector(".txt"),s=p("div","cards"),c.appendChild(s)};try{t.conversationID=await je({apiBase:t.apiBase,service:o,session:t.session,conversationID:t.conversationID,text:a,on:{status:c=>r.set(c||"กำลังทำงาน…"),card:c=>{u(),s.appendChild(Q(c)),k(t.ui.log)},token:c=>{var f;r.remove(),u(),i.textContent=((f=i.textContent)!=null?f:"")+c,k(t.ui.log)}}}),i||y(t.ui.log,"ai",_.empty)}catch(c){const f=c instanceof E?c.message:_.unavailable;y(t.ui.log,"ai",f,"err")}finally{r.remove(),t.busy=!1,t.ui.send.disabled=!1}}async function Ye(e,t,a){var u,g;e.history.push({role:"user",text:a}),e.busy=!0,e.ui.send.disabled=!0;const n=X(e.ui.log,"กำลังส่งคำถาม…");let o=null,r=null,i="";const s=()=>{if(o)return;const c=y(e.ui.log,"ai","");o=c.querySelector(".txt"),r=p("div","cards"),c.appendChild(r)};try{await Fe({apiBase:e.apiBase,token:t.getToken(),messages:e.history.slice(-Ge),page:(g=(u=t.getPage)==null?void 0:u.call(t))!=null?g:"",on:{status:c=>n.set(c||"กำลังทำงาน…"),card:c=>{s(),r.appendChild(Q(c)),k(e.ui.log)},token:c=>{n.remove(),s(),i+=c,o.textContent=i,k(e.ui.log)}}}),i?e.history.push({role:"assistant",text:i}):y(e.ui.log,"ai",_.empty)}catch(c){e.history.pop();const f=c instanceof E?c.message:_.unavailable;y(e.ui.log,"ai",f,"err")}finally{n.remove(),e.busy=!1,e.ui.send.disabled=!1}}function ue(e){var t,a;return e==="light"||e==="dark"?e:(a=(t=window.matchMedia)==null?void 0:t.call(window,"(prefers-color-scheme: dark)"))!=null&&a.matches?"dark":"light"}function Ze(e){return e.replace(/[A-Z]/g,t=>"-"+t.toLowerCase())}function fe(e){Object.defineProperty(window,"__aiOfficeConsole",{value:Object.freeze({mount:a=>z({apiBase:e,console:a}),unmount:P,open:()=>I(!0),close:()=>I(!1),__logText:()=>{var a;return(a=l==null?void 0:l.ui.log.textContent)!=null?a:""},__send:async a=>{l&&(l.ui.input.value=a,await R({}))}}),configurable:!0})}function Xe(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>I(!0),close:()=>I(!1),__hasLauncher:()=>!!l&&l.ui.launcher.style.display!=="none",__isOpen:()=>!!l&&l.ui.panel.dataset.open==="true",__launcherStyle:()=>l?l.ui.launcher.style:{},__config:()=>l?{...l.cfg}:{},__host:()=>{var t;return(t=l==null?void 0:l.host)!=null?t:null},__send:async t=>{l&&(l.ui.input.value=t,await R({}))},__logText:()=>{var t;return(t=l==null?void 0:l.ui.log.textContent)!=null?t:""},__conversationID:()=>{var t;return(t=l==null?void 0:l.conversationID)!=null?t:""}}),configurable:!0})}const O=document.currentScript;if(O){const e={};for(const s in O.dataset)e[s]=O.dataset[s];O.src&&(e.apiBase||(e.apiBase=new URL(O.src,location.href).origin));const t=le((he=e.apiBase)!=null?he:"",e.hostApiBase);let a=null,n=0,o=null;const r=()=>{try{if(!a){if(Date.now()<n)return;n=Date.now()+3e4,q(t).then(u=>{a=u,u&&r()});return}const s=t.readToken()?G(a.service)||"\0":"";if(s===o)return;o=s,s===""?P():z({dataset:e})}catch{}},i=()=>{var s;if(e.consoleMode!==void 0){fe((s=e.apiBase)!=null?s:"");return}if(e.previewMount){z({dataset:e});return}r(),setInterval(r,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",i):i()}return C.__resetSessions=He,C.installConsoleGlobal=fe,C.mount=z,C.unmount=P,Object.defineProperty(C,Symbol.toStringTag,{value:"Module"}),C})({});
