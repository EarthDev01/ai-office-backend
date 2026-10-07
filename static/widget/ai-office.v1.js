var AIOffice=(function(E){"use strict";var Se;const Ae=`
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
.head .x.hist{font-family:var(--font);font-size:12px;padding:3px 4px}
.head .x.hist:disabled{opacity:.4;cursor:default}

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
.datacard.room{display:block;width:100%;text-align:left;cursor:pointer;font:inherit;margin-top:0}
.datacard.room:hover,.datacard.room:focus-visible{border-color:var(--accent)}

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
`;function Ie(e){const t=document.createElement("div");t.setAttribute("data-ai-office-host",""),t.style.all="initial",e?(getComputedStyle(e).position==="static"&&(e.style.position="relative"),t.style.position="absolute",t.style.inset="0",e.appendChild(t)):document.body.appendChild(t);const a=t.attachShadow({mode:"closed"}),n=document.createElement("style");return n.textContent=Ae,a.appendChild(n),{host:t,root:a}}function Te(e){const t=document.createElement("link");t.rel="stylesheet",t.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",e.appendChild(t)}const De=2e3;function Oe(e){const t=u("button","launcher");t.type="button",t.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน"),t.setAttribute("aria-expanded","false"),t.dataset.open="false";const a=u("div","panel");a.dataset.open="false",a.setAttribute("role","dialog"),a.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const n=u("div","head"),o=u("div","avatar"),r=u("div","namewrap"),i=u("div","name"),l=u("div","tag");r.append(i,l);const p=u("div","site");p.title="เว็บที่กำลังคุยอยู่";const f=u("button","x hist");f.type="button",f.textContent="ประวัติ",f.title="ห้องแชทย้อนหลัง 7 วัน",f.setAttribute("aria-label","ห้องแชทย้อนหลัง 7 วัน");const s=u("button","x");s.type="button",s.textContent="×",s.setAttribute("aria-label","ปิด"),n.append(o,r,p,f,s);const h=u("div","log");h.setAttribute("aria-live","polite");const m=u("div","foot"),d=u("textarea");d.rows=1,d.maxLength=De,d.placeholder="พิมพ์คำถาม…",d.setAttribute("aria-label","คำถาม");const g=u("button","send");g.type="button",g.textContent="↑",g.setAttribute("aria-label","ส่ง"),m.append(d,g),a.append(n,h,m),e.append(t,a);const b={launcher:t,panel:a,head:{avatar:o,name:i,tag:l,site:p,history:f},log:h,input:d,send:g};return s.addEventListener("click",()=>V(b,!1)),b}function V(e,t){e.panel.dataset.open=String(t),e.launcher.dataset.open=String(t),e.launcher.setAttribute("aria-expanded",String(t)),e.launcher.setAttribute("aria-label",t?"ปิดผู้ช่วยหลังบ้าน":"เปิดผู้ช่วยหลังบ้าน")}const re="http://www.w3.org/2000/svg";function K(e,t){const a=document.createElementNS(re,"svg");a.setAttribute("viewBox","0 0 24 24"),a.setAttribute("aria-hidden","true"),a.setAttribute("class",e);for(const n of t){const o=document.createElementNS(re,"path");o.setAttribute("d",n.d),n.fill&&o.setAttribute("class","fill"),a.appendChild(o)}return a}const ie=[{d:"M3.5 6.25A3.25 3.25 0 0 1 6.75 3h10.5a3.25 3.25 0 0 1 3.25 3.25v7.5A3.25 3.25 0 0 1 17.25 17H11.5l-4.1 3.4c-.7.58-1.9.1-1.9-.83V17A3.25 3.25 0 0 1 3.5 13.75z"},{d:"M12 5.6l1.05 2.6 2.6 1.05-2.6 1.05L12 12.9l-1.05-2.6-2.6-1.05 2.6-1.05z",fill:!0}],$e=[{d:"M7 7l10 10M17 7L7 17"}];function Be(e,t){e.launcher.textContent="";const a=u("span","face");if(t){e.launcher.dataset.icon="true";const n=u("span","licon");n.style.setProperty("--licon",`url("${t}")`),n.append(de(t),u("span","tint")),a.appendChild(n)}else delete e.launcher.dataset.icon,a.appendChild(K("ico",ie));e.launcher.append(a,K("ico close",$e))}function y(e,t,a,n=""){const o=u("div","row "+t),r=u("div","bubble"+(n?" "+n:"")),i=u("div","txt");return i.textContent=a,r.appendChild(i),o.appendChild(r),e.appendChild(o),C(e),r}function L(e,t){const a=u("div","sys");return a.textContent=t,e.appendChild(a),C(e),a}function J(e,t){const a=u("div","row ai"),n=u("div","bubble load"),o=u("span");o.textContent=t;const r=u("span","dots");return r.append(u("span"),u("span"),u("span")),n.append(o,r),a.appendChild(n),e.appendChild(a),C(e),{set:i=>o.textContent=i,remove:()=>a.remove()}}function se(e,t,a){const n=u("button","datacard room");n.type="button";const o=u("div","dc-title");if(o.textContent=e,n.appendChild(o),t){const r=u("div","src");r.textContent=t,n.appendChild(r)}return n.addEventListener("click",a),n}function q(e){var r,i,l,p,f;const t=u("div","datacard k-"+je(e.kind));if(e.title){const s=u("div","dc-title");s.textContent=e.title,t.appendChild(s)}for(const s of(r=e.fields)!=null?r:[]){const h=u("div","dc-row"),m=u("span");m.textContent=s.label;const d=u("b");d.textContent=s.display,h.append(m,d),t.appendChild(h)}if(e.table&&((i=e.table.rows)!=null&&i.length)){const s=u("div","dc-tablewrap"),h=u("table","dc-table"),m=u("thead"),d=u("tr");for(const b of(l=e.table.columns)!=null?l:[]){const x=u("th");x.textContent=b.label,d.appendChild(x)}m.appendChild(d);const g=u("tbody");for(const b of e.table.rows){const x=u("tr");for(const k of b){const _=u("td");_.textContent=(p=k==null?void 0:k.display)!=null?p:"",x.appendChild(_)}g.appendChild(x)}h.append(m,g),s.appendChild(h),t.appendChild(s)}if(e.note){const s=u("div","dc-note");s.textContent=e.note,t.appendChild(s)}const a=u("div","src"),n=u("span");n.textContent=e.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+ce(e.fetched_at)+(e.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),a.appendChild(n);const o=Re((f=e.link)==null?void 0:f.path);if(e.link&&o){const s=u("a");s.href=o,s.textContent=(e.link.label||"เปิดหน้าจริง")+" →",s.target="_self",s.rel="noopener",a.appendChild(s)}return t.appendChild(a),t}function ce(e){const t=new Date(e);if(isNaN(t.getTime()))return"-";const a={timeZone:"Asia/Bangkok"};try{const n=t.toLocaleTimeString("th-TH",{...a,hour:"2-digit",minute:"2-digit",hour12:!1}),o=t.toLocaleDateString("en-CA",a),r=new Date().toLocaleDateString("en-CA",a);if(o===r)return n;const[,i,l]=o.split("-");return`${l}/${i} ${n}`}catch{return t.toISOString().slice(0,16).replace("T"," ")}}function Re(e){if(!e)return"";const t=e.trim();return t.startsWith("#")?t:!t.startsWith("/")||t.startsWith("//")||t.includes("\\")?"":t}function Le(e,t){const{position:a,offset_x:n,offset_y:o}=t.placement,r=a==="bottom-left";for(const i of[e.launcher,e.panel])i.style.left="",i.style.right="";e.launcher.style.bottom=A(o),e.panel.style.bottom=A(o+68),r?(e.launcher.style.left=A(n),e.panel.style.left=A(n)):(e.launcher.style.right=A(n),e.panel.style.right=A(n))}function Ne(e,t,a="",n=!0){var r,i;e.head.name.textContent=t.display_name||(n?"ผู้ช่วยหลังบ้าน":"ผู้ช่วย"),e.head.tag.textContent=(r=t.tagline)!=null?r:"",e.head.site.textContent=n&&(t.service_label||t.service_id)||"",e.head.site.style.display=e.head.site.textContent?"":"none";const o=Y(t.avatar_url,a);e.head.avatar.textContent="",o?e.head.avatar.appendChild(de(o)):e.head.avatar.appendChild(K("ico",ie)),Be(e,Y(t.launcher_icon,a)),o||t.tagline||t.background?e.panel.dataset.rich="true":delete e.panel.dataset.rich,Me(e.log,(i=t.background)!=null?i:"",a),e.log.dataset.bg==="image"?e.panel.dataset.bgimg="true":delete e.panel.dataset.bgimg}const Pe=/^(https:\/\/|http:\/\/localhost(:\d+)?\/)[A-Za-z0-9._~:/?#[\]@!$&*+,;=%-]+$/,ze=new Set(["dots","grid","diagonal","glow"]);function Y(e,t){if(!e)return"";if(e.startsWith("asset:")){const a=e.slice(6);return/^[a-z0-9_-]+\/[A-Za-z0-9._-]+$/.test(a)?`${t}/widget/v1/assets/${a}`:""}return Pe.test(e)?e:""}function Me(e,t,a){if(e.style.removeProperty("--log-img"),delete e.dataset.bg,t.startsWith("pattern:")){const o=t.slice(8);ze.has(o)&&(e.dataset.bg=o);return}const n=Y(t,a);n&&(e.dataset.bg="image",e.style.setProperty("--log-img",`url("${n}")`))}const le=/^#[0-9a-f]{6}$/i;function X(e,t,a,n=""){const o=["--accent","--accent-stops","--accent-soft","--on-accent"],r=(Array.isArray(t)?t:t?[t]:[]).filter(d=>le.test(d)).slice(0,4);if(!r.length){for(const d of o)e.style.removeProperty(d);return}const i=r[0];r.length===1&&r.push(i);const l=r.map(d=>[1,3,5].map(g=>parseInt(d.slice(g,g+2),16))),p=l[0],f=[0,1,2].map(d=>l.reduce((g,b)=>g+b[d],0)/l.length),s=a==="dark"?[21,33,35]:[255,255,255],h=d=>"#"+p.map((g,b)=>Math.round(g*d+s[b]*(1-d)).toString(16).padStart(2,"0")).join(""),m=f.map(d=>d/255).map(d=>d<=.03928?d/12.92:((d+.055)/1.055)**2.4).reduce((d,g,b)=>d+g*[.2126,.7152,.0722][b],0);e.style.setProperty("--accent",i),e.style.setProperty("--accent-stops",r.join(", ")),e.style.setProperty("--accent-soft",h(a==="dark"?.25:.16)),e.style.setProperty("--on-accent",le.test(n)?n:m>.45?"#13282B":"#ffffff")}function Ue(e){if(!e)return null;const t=getComputedStyle(document.documentElement),a=document.createElement("span");a.style.display="none",document.body.appendChild(a);const n=r=>{if(!r||!/^--[A-Za-z0-9_-]+$/.test(r))return"";const i=t.getPropertyValue(r).trim();if(!i||(a.style.color="",a.style.color=i,!a.style.color))return"";const l=/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?/.exec(getComputedStyle(a).color);return!l||l[4]!==void 0&&Number(l[4])===0?"":"#"+[l[1],l[2],l[3]].map(p=>Number(p).toString(16).padStart(2,"0")).join("")},o={accent:n(e.accent),accent2:n(e.accent_2),on:n(e.on_accent)};return a.remove(),o.accent?o:null}function C(e){e.scrollTop=e.scrollHeight}function de(e){const t=document.createElement("img");return t.src=e,t.alt="",t.referrerPolicy="no-referrer",t}function je(e){return String(e||"").replace(/[^a-z_]/g,"")}function A(e){return`${e|0}px`}function u(e,t){const a=document.createElement(e);return t&&(a.className=t),a}const Fe=["avatar_url","display_name","greeting","tagline","launcher_icon","background","theme","accent_color","accent_colors","color_source","placement","service_label","is_hidden"],He=2*1024*1024;function Ge(e,t,a){if(!e||typeof t!="string"||!t.startsWith("/")||t.startsWith("//")||t.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(t)||t.includes("://"))return null;let n;try{n=new URL(e)}catch{return null}const o=n.pathname.replace(/\/+$/,"");let r;try{r=new URL(n.origin+o+t)}catch{return null}if(r.origin!==n.origin||!(r.pathname===o||r.pathname.startsWith(o+"/")))return null;if(a)for(const[i,l]of Object.entries(a))r.searchParams.set(i,String(l));return r.toString()}async function ue(e,t,a,n="Bearer"){var p;if(!a)return{status:401,body:""};const o=String(t.method||"").toUpperCase();if(o!=="GET"&&o!=="POST")return{status:0,body:""};const r=Ge(e,t.path,t.query);if(!r)return{status:0,body:""};const i=new AbortController,l=setTimeout(()=>i.abort(),Math.max(1e3,(p=t.timeout_ms)!=null?p:2e4));try{const f={method:o,credentials:"omit",signal:i.signal,headers:{Authorization:`${n} ${a}`,Accept:"application/json"}};o==="POST"&&t.body!==void 0&&t.body!==null&&(f.headers["Content-Type"]="application/json",f.body=JSON.stringify(t.body));const s=await fetch(r,f),h=await s.text();return h.length>He?{status:0,body:""}:{status:s.status,body:h}}catch{return{status:0,body:""}}finally{clearTimeout(l)}}const D="ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง";class v extends Error{constructor(t,a=""){super(t),this.code=a}}const We={SERVICE_DISABLED:"ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่",CHAT_DISABLED:"ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่",LLM_NOT_CONFIGURED:"ผู้ช่วยยังไม่พร้อมใช้งาน — ผู้ดูแลระบบยังตั้งค่าไม่เสร็จ",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์ใช้ผู้ช่วยในเว็บนี้",SESSION_EXPIRED:"การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่",NOT_AUTHENTICATED:"การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่",BACKOFFICE_UNAVAILABLE:"ตรวจสอบสิทธิ์กับหลังบ้านไม่ได้ชั่วคราว กรุณาลองใหม่อีกครั้ง",TICKET_INVALID:"การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า",TICKET_SERVICE_MISMATCH:"การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า",ORIGIN_NOT_REGISTERED:"ผู้ช่วยยังไม่ได้เปิดให้หลังบ้านนี้"},Ve=new Set(["SERVICE_DISABLED","CHAT_DISABLED","SERVICE_NOT_ALLOWED","ORIGIN_NOT_REGISTERED"]),Z=new Set(["TICKET_INVALID","TICKET_SERVICE_MISMATCH"]);async function O(e,t){var o,r;const a=await e.json().catch(()=>null),n=(o=a==null?void 0:a.message)!=null?o:"";return new v((r=We[n])!=null?r:t,n)}async function Ke(e){const{apiBase:t,service:a,session:n,on:o}=e,r=`${t}/api/ai/widget/service/${encodeURIComponent(a)}`;let i=e.conversationID;const l=async d=>fetch(`${r}/chat`,{method:"POST",headers:{Authorization:`Bearer ${d}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:i,text:e.text}),signal:e.signal});let p=await n.ticket(a),f=await l(p);if(!f.ok||!f.body){let d=await O(f,D);if(Z.has(d.code)&&(n.invalidate(),p=await n.ticket(a),f=await l(p),d=f.ok&&f.body?null:await O(f,D)),d)throw Z.has(d.code)&&n.invalidate(),d}const s=[],h=d=>{var g;return!((g=e.signal)!=null&&g.aborted)&&s.push(ue(n.hostApiBase(),d,n.readToken(),n.authScheme()).then(b=>fetch(`${r}/chat/relay/${encodeURIComponent(d.id)}`,{method:"POST",headers:{Authorization:`Bearer ${p}`,"Content-Type":"application/json"},body:JSON.stringify(b)})).then(()=>{}).catch(()=>{}))};let m=null;if(await fe(f.body,(d,g)=>{var x,k,_,R;const b=g;switch(d){case"status":typeof b.conversation_id=="string"&&(i=b.conversation_id),o.status(String((x=b.text)!=null?x:""));break;case"fetch":h(b);break;case"card":o.card(b);break;case"token":o.token(String((k=b.text)!=null?k:""));break;case"error":m=new v(String((_=b.message)!=null?_:"ผู้ช่วยตอบไม่สำเร็จ"),String((R=b.code)!=null?R:""));break}}),await Promise.all(s),m)throw m;return i}async function pe(e,t,a,n){const o=`${e}/api/ai/widget/service/${encodeURIComponent(t)}${n}`,r=async()=>fetch(o,{headers:{Authorization:`Bearer ${await a.ticket(t)}`}});let i=await r();if(!i.ok){let p=await O(i,D);if(Z.has(p.code)&&(a.invalidate(),i=await r(),p=i.ok?null:await O(i,D)),p)throw p}const l=await i.json().catch(()=>null);if(!(l!=null&&l.payload))throw new v(D);return l.payload}function Je(e,t,a,n=7){return pe(e,t,a,`/conversations?days=${n}`)}function qe(e,t,a,n){return pe(e,t,a,`/conversations/${encodeURIComponent(n)}`)}async function Ye(e){const t=await fetch(`${e.apiBase}/api/ai/admin/assistant`,{method:"POST",headers:{Authorization:`Bearer ${e.token}`,"Content-Type":"application/json"},body:JSON.stringify({messages:e.messages,page:e.page})});if(!t.ok||!t.body){const n=await t.json().catch(()=>null);throw new v((n==null?void 0:n.error)||`เซิร์ฟเวอร์ตอบ ${t.status}`)}let a=null;if(await fe(t.body,(n,o)=>{var i,l,p;const r=o;n==="status"?e.on.status(String((i=r.text)!=null?i:"")):n==="card"?e.on.card(r):n==="token"?e.on.token(String((l=r.text)!=null?l:"")):n==="error"&&(a=new v(String((p=r.message)!=null?p:"ผู้ช่วยตอบไม่สำเร็จ")))}),a)throw a}async function fe(e,t){const a=e.getReader(),n=new TextDecoder;let o="";for(;;){const{value:r,done:i}=await a.read();if(i)break;o+=n.decode(r,{stream:!0});let l;for(;(l=o.indexOf(`

`))>=0;){const p=o.slice(0,l);o=o.slice(l+2);let f="message",s="";for(const h of p.split(`
`))h.startsWith("event:")?f=h.slice(6).trim():h.startsWith("data:")&&(s+=h.slice(5).trim());s&&t(f,JSON.parse(s))}}}const I=new Set(["","null","undefined","false"]);function Q(e){try{if(e==="localStorage")return localStorage;if(e==="sessionStorage")return sessionStorage}catch{}return null}function he(e){var t,a;if(!e)return"";try{const n=(a=(t=Q(e.source))==null?void 0:t.getItem(Xe(e)))!=null?a:"";if(I.has(n.trim()))return"";if(e.format!=="json-expiration")return ge(n);const o=JSON.parse(n),r=o==null?void 0:o[e.value_field||"value"];if(typeof r!="string"||I.has(r))return"";const i=o[e.expiration_field||"expiration"];return typeof i=="number"&&i<=Math.floor(Date.now()/1e3)?"":ge(r)}catch{return""}}function Xe(e){var a,n,o;if(!e.key_from)return e.key;const t=((n=(a=Q(e.key_from.source))==null?void 0:a.getItem(e.key_from.key))!=null?n:"").trim();return e.key.split("{key_from}").join(I.has(t)?(o=e.key_from.default)!=null?o:"":t)}function ge(e){const t=e.trim(),a=/^bearer(\s+|$)/i.exec(t),n=a?t.slice(a[0].length).trim():t;return I.has(n)?"":n}function ee(e){var t,a,n,o,r;if(!e)return"";if(e.source==="office")return(t=e.value)!=null?t:"";try{let i="";if(e.source==="query"){if(i=(a=new URLSearchParams(location.search).get(e.key))!=null?a:"",!i){const l=location.hash.indexOf("?");l>=0&&(i=(n=new URLSearchParams(location.hash.slice(l+1)).get(e.key))!=null?n:"")}}else i=(r=(o=Q(e.source))==null?void 0:o.getItem(e.key))!=null?r:"";if(i=i.trim(),I.has(i))return"";if(e.encoding==="base64"){const l=atob(i).trim();return I.has(l)?"":l}return i}catch{return""}}function be(e){return e?e.source==="office"?"service ของ domain นี้ (เปิดอย่างน้อย 1 service ที่คอนโซล)":e.source==="query"?`?${e.key}=`:`${e.source}["${e.key}"]`:"(ไม่ทราบ)"}function Ze(e){const t=e.split(".")[1];if(!t)return null;try{const a=t.replace(/-/g,"+").replace(/_/g,"/"),n=atob(a+"=".repeat((4-a.length%4)%4)),o=Uint8Array.from(n,r=>r.charCodeAt(0));return JSON.parse(new TextDecoder().decode(o))}catch{return null}}function me(e,t){const a=N(e,t.path);if(!Array.isArray(a))return[];const n=[];for(const o of a){if(t.where_field&&N(o,t.where_field)!==t.where_value)continue;const r=t.pluck?N(o,t.pluck):o;typeof r=="string"&&r&&n.push(r)}return n}function N(e,t){let a=e;if(!t)return a;for(const n of t.split(".")){if(!a||typeof a!="object")return;a=a[n]}return a}const Qe=180*1e3;class et{constructor(t,a=""){this.apiBase=t,this.hostAPIBase=a,this.cfg=null,this.loading=null,this.cur=null}hostApiBase(){var t;return this.hostAPIBase||((t=this.cfg)==null?void 0:t.host_api_base)||""}readToken(){var t;return he((t=this.cfg)==null?void 0:t.token)}authScheme(){var t;return((t=this.cfg)==null?void 0:t.auth_scheme)||"Bearer"}async pageConfig(){var t;return this.cfg?this.cfg:((t=this.loading)!=null||(this.loading=(async()=>{var o;const a=await fetch(`${this.apiBase}/api/ai/widget/page-config`),n=await a.json().catch(()=>null);if(!a.ok||!(n!=null&&n.payload))throw new ye((o=n==null?void 0:n.message)!=null?o:String(a.status));return this.cfg=n.payload,this.cfg})().finally(()=>{this.loading=null})),this.loading)}async ticket(t){var i;if(this.cur&&this.cur.service===t&&this.cur.expiresAt-Date.now()>Qe)return this.cur.ticket;const a=await this.pageConfig(),n=await this.readPermissions(a,t),o=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(t)}/browser-session`,{method:"POST",headers:{Authorization:`Bearer ${this.readToken()}`,"Content-Type":"application/json"},body:JSON.stringify({permissions:n})});if(!o.ok)throw await O(o,`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${o.status})`);const r=await o.json().catch(()=>null);if(!((i=r==null?void 0:r.payload)!=null&&i.ticket))throw new v(`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${o.status})`);return this.cur={service:t,ticket:r.payload.ticket,expiresAt:Date.now()+r.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(t,a){const n=t.identity,o=n==null?void 0:n.permissions;if(!(o!=null&&o.path))return[];if(n!=null&&n.permissions_token){const l=he(n.permissions_token);return l?me(N(Ze(l),n.root),o):[]}const r=n==null?void 0:n.permissions_request;if(!r)return[];const i=await ue(this.hostApiBase(),{id:"perm",method:"GET",path:r.split("{service}").join(encodeURIComponent(a))},this.readToken(),this.authScheme());if(i.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${i.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{return me(JSON.parse(i.body),o)}catch{return[]}}}class ye extends Error{}const tt=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],te=new Map;function at(){te.clear()}function xe(e,t=""){const a=e+"|"+t;let n=te.get(a);return n||(n=new et(e,t),te.set(a,n)),n}const P={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}},nt=20;let c=null;function z(){var e;window.removeEventListener("message",ve),H(),(e=c==null?void 0:c.abort)==null||e.abort(),c==null||c.host.remove(),c=null}async function M(e={}){var m,d,g,b,x,k,_,R,Ee;z();const t=(m=e.dataset)!=null?m:{};for(const S of tt)t[S]&&(console.warn(`[ai-office] ไม่รับ data-${pt(S)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete t[S]);const a=(d=t.previewMount)!=null?d:"",n=a!=="",o=(b=(g=e.apiBase)!=null?g:t.apiBase)!=null?b:"",r=xe(o,t.hostApiBase);let i;if(e.console)i={...P,display_name:(x=e.console.displayName)!=null?x:"ผู้ช่วย AI Office",greeting:(k=e.console.greeting)!=null?k:"สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)",service_label:(_=e.console.label)!=null?_:"คอนโซล",placement:{position:"bottom-right",offset_x:20,offset_y:20}};else if(n)i={...P,...(R=e.bootstrap)!=null?R:{}};else if(e.bootstrap)i={...P,...e.bootstrap};else{const S=await ot(o,r,e.onFetch);if(!S)return;i={...P,...S}}if(!n&&!i.enabled)return;const l=n?document.querySelector(a):null;if(n&&!l){console.warn(`[ai-office] ไม่พบกล่อง preview: ${a}`);return}const{host:p,root:f}=Ie(l);n&&p.setAttribute("data-preview","true"),Te(f);const s=Oe(f),h=!n&&!e.console&&!e.bootstrap?await r.pageConfig().catch(()=>null):null;c={ui:s,host:p,cfg:i,preview:n,apiBase:o,session:r,conversationID:"",busy:!1,console:(Ee=e.console)!=null?Ee:null,history:[],abort:null,listing:!1,showSite:(h==null?void 0:h.audience)!=="player",pageColors:h==null?void 0:h.page_colors},(n||e.console)&&(s.head.history.style.display="none"),U(i),s.launcher.addEventListener("click",()=>$()),s.head.history.addEventListener("click",()=>{we()}),s.send.addEventListener("click",()=>G(e)),s.input.addEventListener("keydown",S=>{const W=S;W.key==="Enter"&&!W.shiftKey&&!W.isComposing&&(W.preventDefault(),G(e))}),n&&(V(s,!0),window.addEventListener("message",ve)),e.console||ft()}function U(e){if(!c)return;c.cfg=e;const t=Ce(e.theme);c.host.setAttribute("data-theme",t);const a=ne(t);Ne(c.ui,e,c.apiBase,c.showSite),Le(c.ui,e),c.ui.launcher.style.display=e.is_hidden?"none":"grid",c.ui.log.textContent="",e.greeting&&y(c.ui.log,"ai",e.greeting),c.preview&&L(c.ui.log,w.preview),a||st()}function ve(e){if(e.origin!==window.location.origin)return;const t=e.data;if(!t||t.type!=="ai-office:preview-config"||!t.config||!c||!c.preview)return;const a={};for(const n of Fe)n in t.config&&(a[n]=t.config[n]);U({...c.cfg,...a})}async function ot(e,t,a){var l,p,f,s,h;const n=await ae(t);if(!n)return null;const o=t.readToken(),r=ee(n.service);if(!o||!r){const m=be(n.token),d=be(n.service);return T(!o&&!r?`ไม่พบ ${m} และ ${d} — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน`:o?`ไม่พบ ${d} — ยังไม่ได้เลือกเว็บในหลังบ้าน (หน้านี้ไม่ได้ผูกกับเว็บใด)`:`ไม่พบ ${m} ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว`),null}const i=`${e}/api/ai/widget/service/${encodeURIComponent(r)}/bootstrap`;a==null||a({url:i});try{const m=await fetch(i,{headers:{Authorization:`Bearer ${o}`}}),d=await m.json().catch(()=>null);if(!m.ok)return T(`เซิร์ฟเวอร์ตอบ ${m.status} ${(l=d==null?void 0:d.message)!=null?l:""} — ${(p=d==null?void 0:d.error)!=null?p:""}`.trim(),j[(f=d==null?void 0:d.message)!=null?f:""]),null;const g=(s=d==null?void 0:d.payload)!=null?s:null;if(!g)return null;if(!g.enabled)return T(`ยังไม่เปิดใช้งาน (reason: ${g.reason})`,j[(h=g.reason)!=null?h:""]),g;try{await t.ticket(r)}catch(b){const x=b instanceof v?b.code:"";return T(`ขอตั๋วแชทไม่สำเร็จ (${x||b.message})`,j[x]),null}return g}catch(m){return T(`เรียก ${i} ไม่สำเร็จ — ${m.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}async function ae(e){var t;try{return await e.pageConfig()}catch(a){const n=a instanceof ye?a.message:"";return T(`โหลดการตั้งค่าของหน้านี้ไม่สำเร็จ (${n||a.message})`,(t=j[n])!=null?t:"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function T(e,t){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${e}`+(t?`
           → ${t}`:""))}const j={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์เปิดเว็บนี้ในหลังบ้าน (รายชื่อเว็บของบัญชี)",KIND_NOT_SUPPORTED:'ชนิดหลังบ้านของ domain นี้ยังไม่มี connector — ตรวจ "ชนิดหลังบ้าน" ที่คอนโซล',SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function ne(e=Ce((t=>(t=c==null?void 0:c.cfg.theme)!=null?t:"auto")())){var o;if(!c)return!0;const a=c.cfg,n=a.color_source==="site"?Ue(c.pageColors):null;return n?X(c.host,[n.accent,n.accent2||n.accent],e,n.on):a.color_source==="site"?X(c.host,void 0,e):X(c.host,(o=a.accent_colors)!=null&&o.length?a.accent_colors:a.accent_color,e),a.color_source!=="site"||!!n}const rt=4e3,it=2e4;let F=null;function st(){H();const e=c;if(!e||e.preview)return;const t=Date.now();e.ui.launcher.style.visibility="hidden",F=setInterval(()=>{if(c!==e)return H();const a=ne(),n=Date.now()-t;(a||n>rt)&&(e.ui.launcher.style.visibility=""),(a||n>it)&&H()},250)}function H(){F&&clearInterval(F),F=null}function $(e){if(!c)return;const t=e!=null?e:c.ui.panel.dataset.open!=="true";t&&ne(),V(c.ui,t),t&&c.ui.input.focus()}const w={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)",rooms:"ห้องแชทย้อนหลัง 7 วัน — แตะเพื่อเปิดคุยต่อ",noRooms:"ยังไม่มีห้องแชทใน 7 วันนี้ · ห้องจากการล็อกอินครั้งก่อนเปิดจากที่นี่ไม่ได้ (เพื่อความปลอดภัย)"};async function G(e){var f;const t=c;if(!t||t.busy||t.ui.input.disabled)return;const a=t.ui.input.value.trim();if(!a)return;if(t.ui.input.value="",y(t.ui.log,"me",a),t.preview){y(t.ui.log,"ai",w.preview);return}if(t.console){await ut(t,t.console,a);return}const n=await oe(t);if(!n){y(t.ui.log,"ai",w.unavailable,"err");return}t.listing&&(t.listing=!1,t.conversationID="",U(t.cfg),y(t.ui.log,"me",a)),(f=e.onFetch)==null||f.call(e,{url:`${t.apiBase}/api/ai/widget/service/${encodeURIComponent(n)}/chat`,body:{text:a}}),t.busy=!0,t.ui.send.disabled=!0,t.ui.head.history.disabled=!0;const o=t.abort=new AbortController,r=J(t.ui.log,"กำลังส่งคำถาม…");let i=null,l=null;const p=()=>{if(i)return;const s=y(t.ui.log,"ai","");i=s.querySelector(".txt"),l=u("div","cards"),s.appendChild(l)};try{t.conversationID=await Ke({apiBase:t.apiBase,service:n,session:t.session,conversationID:t.conversationID,text:a,signal:o.signal,on:{status:s=>r.set(s||"กำลังทำงาน…"),card:s=>{p(),l.appendChild(q(s)),C(t.ui.log)},token:s=>{var h;r.remove(),p(),i.textContent=((h=i.textContent)!=null?h:"")+s,C(t.ui.log)}}}),i||y(t.ui.log,"ai",w.empty)}catch(s){if(o.signal.aborted)return;const h=s instanceof v?s.message:w.unavailable;y(t.ui.log,"ai",h,"err");const m=s instanceof v?s.code:"";m==="not_found"&&(t.conversationID=""),Ve.has(m)&&dt(t)}finally{r.remove(),t.abort=null,t.busy=!1,t.ui.send.disabled=t.ui.input.disabled,t.ui.head.history.disabled=!1}}async function oe(e){const t=await ae(e.session);return t?ee(t.service):""}async function we(){const e=c;if(!e||e.busy||e.preview||e.console)return;const t=await oe(e);if(!t)return;e.busy=!0,e.ui.head.history.disabled=!0;const a=J(e.ui.log,"กำลังโหลดประวัติ…");try{const n=await Je(e.apiBase,t,e.session,7);if(c!==e)return;e.listing=!0,e.ui.log.textContent="",L(e.ui.log,w.rooms),e.ui.log.appendChild(se("＋ เริ่มห้องใหม่","",()=>ct(e)));for(const o of n)e.ui.log.appendChild(se(o.title||"(ไม่มีหัวข้อ)","คุยล่าสุด "+ce(o.updated_at),()=>{ke(e,t,o.id)}));n.length||L(e.ui.log,w.noRooms),C(e.ui.log)}catch(n){y(e.ui.log,"ai",n instanceof v?n.message:w.unavailable,"err")}finally{a.remove(),e.busy=!1,e.ui.head.history.disabled=!1}}function ct(e){e.listing=!1,e.conversationID="",U(e.cfg),e.ui.input.focus()}async function ke(e,t,a){if(!e.busy){e.busy=!0;try{const n=await qe(e.apiBase,t,e.session,a);if(c!==e)return;e.listing=!1,e.conversationID=n.id,e.ui.log.textContent="",L(e.ui.log,`ห้องเดิม: ${n.title} · ตัวเลขในการ์ดเป็นค่า ณ เวลาที่ดึง ถามใหม่เพื่อดูค่าล่าสุด`);for(const o of n.messages)lt(e,o);e.ui.input.focus()}catch(n){y(e.ui.log,"ai",n instanceof v?n.message:w.unavailable,"err")}finally{e.busy=!1}}}function lt(e,t){var n,o;if(t.role==="user"){y(e.ui.log,"me",t.text);return}if(!t.text&&!((n=t.cards)!=null&&n.length))return;const a=y(e.ui.log,"ai",t.text,t.status==="error"?"err":"");if((o=t.cards)!=null&&o.length){const r=u("div","cards");for(const i of t.cards)r.appendChild(q(i));a.appendChild(r)}}function dt(e){e.ui.input.disabled=!0,e.ui.send.disabled=!0,e.ui.launcher.style.display="none"}async function ut(e,t,a){var p,f;e.history.push({role:"user",text:a}),e.busy=!0,e.ui.send.disabled=!0;const n=J(e.ui.log,"กำลังส่งคำถาม…");let o=null,r=null,i="";const l=()=>{if(o)return;const s=y(e.ui.log,"ai","");o=s.querySelector(".txt"),r=u("div","cards"),s.appendChild(r)};try{await Ye({apiBase:e.apiBase,token:t.getToken(),messages:e.history.slice(-nt),page:(f=(p=t.getPage)==null?void 0:p.call(t))!=null?f:"",on:{status:s=>n.set(s||"กำลังทำงาน…"),card:s=>{l(),r.appendChild(q(s)),C(e.ui.log)},token:s=>{n.remove(),l(),i+=s,o.textContent=i,C(e.ui.log)}}}),i?e.history.push({role:"assistant",text:i}):y(e.ui.log,"ai",w.empty)}catch(s){e.history.pop();const h=s instanceof v?s.message:w.unavailable;y(e.ui.log,"ai",h,"err")}finally{n.remove(),e.busy=!1,e.ui.send.disabled=!1}}function Ce(e){var t,a;return e==="light"||e==="dark"?e:(a=(t=window.matchMedia)==null?void 0:t.call(window,"(prefers-color-scheme: dark)"))!=null&&a.matches?"dark":"light"}function pt(e){return e.replace(/[A-Z]/g,t=>"-"+t.toLowerCase())}function _e(e){Object.defineProperty(window,"__aiOfficeConsole",{value:Object.freeze({mount:a=>M({apiBase:e,console:a}),unmount:z,open:()=>$(!0),close:()=>$(!1),__logText:()=>{var a;return(a=c==null?void 0:c.ui.log.textContent)!=null?a:""},__send:async a=>{c&&(c.ui.input.value=a,await G({}))}}),configurable:!0})}function ft(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>$(!0),close:()=>$(!1),__hasLauncher:()=>!!c&&c.ui.launcher.style.display!=="none",__isOpen:()=>!!c&&c.ui.panel.dataset.open==="true",__launcherStyle:()=>c?c.ui.launcher.style:{},__config:()=>c?{...c.cfg}:{},__host:()=>{var t;return(t=c==null?void 0:c.host)!=null?t:null},__send:async t=>{c&&(c.ui.input.value=t,await G({}))},__logText:()=>{var t;return(t=c==null?void 0:c.ui.log.textContent)!=null?t:""},__conversationID:()=>{var t;return(t=c==null?void 0:c.conversationID)!=null?t:""},__showRooms:()=>we(),__openRoom:async t=>{if(!c)return;const a=await oe(c);await ke(c,a,t)}}),configurable:!0})}const B=document.currentScript;if(B){const e={};for(const l in B.dataset)e[l]=B.dataset[l];B.src&&(e.apiBase||(e.apiBase=new URL(B.src,location.href).origin));const t=xe((Se=e.apiBase)!=null?Se:"",e.hostApiBase);let a=null,n=0,o=null;const r=()=>{try{if(!a){if(Date.now()<n)return;n=Date.now()+3e4,ae(t).then(p=>{a=p,p&&r()});return}const l=t.readToken()?ee(a.service)||"\0":"";if(l===o)return;o=l,l===""?z():M({dataset:e})}catch{}},i=()=>{var l;if(e.consoleMode!==void 0){_e((l=e.apiBase)!=null?l:"");return}if(e.previewMount){M({dataset:e});return}r(),setInterval(r,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",i):i()}return E.__resetSessions=at,E.installConsoleGlobal=_e,E.mount=M,E.unmount=z,Object.defineProperty(E,Symbol.toStringTag,{value:"Module"}),E})({});
