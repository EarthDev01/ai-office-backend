var AIOffice=(function(y){"use strict";const W=`
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

.head .namewrap{min-width:0;flex:1 1 auto}

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
.row.me .bubble{background:var(--accent);color:#fff;border-color:transparent}
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
  background:var(--accent);color:#fff;font-family:var(--font-head);font-weight:600;
}
.foot .send:disabled,.foot textarea:disabled{opacity:.5;cursor:default}

/* ---- โหมด preview: อยู่ในกล่อง ไม่ลอยมุมจอ ---- */
:host([data-preview="true"]) .launcher,
:host([data-preview="true"]) .panel{position:absolute}
:host([data-preview="true"]) .panel{
  inset:auto; top:12px; left:12px; right:12px; bottom:12px;
  width:auto; height:auto; display:flex;
}
`;function H(t){const e=document.createElement("div");e.setAttribute("data-ai-office-host",""),e.style.all="initial",t?(getComputedStyle(t).position==="static"&&(t.style.position="relative"),e.style.position="absolute",e.style.inset="0",t.appendChild(e)):document.body.appendChild(e);const n=e.attachShadow({mode:"closed"}),a=document.createElement("style");return a.textContent=W,n.appendChild(a),{host:e,root:n}}function j(t){const e=document.createElement("link");e.rel="stylesheet",e.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",t.appendChild(e)}const G=2e3;function J(t){const e=c("button","launcher");e.type="button",e.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน");const n=c("div","panel");n.dataset.open="false",n.setAttribute("role","dialog"),n.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const a=c("div","head"),i=c("div","avatar"),o=c("div","namewrap"),l=c("div","name");o.appendChild(l);const f=c("div","site");f.title="เว็บที่กำลังคุยอยู่";const u=c("button","x");u.type="button",u.textContent="×",u.setAttribute("aria-label","ปิด"),a.append(i,o,f,u);const p=c("div","log");p.setAttribute("aria-live","polite");const r=c("div","foot"),d=c("textarea");d.rows=1,d.maxLength=G,d.placeholder="พิมพ์คำถาม…",d.setAttribute("aria-label","คำถาม");const h=c("button","send");return h.type="button",h.textContent="↑",h.setAttribute("aria-label","ส่ง"),r.append(d,h),n.append(a,p,r),t.append(e,n),u.addEventListener("click",()=>n.dataset.open="false"),{launcher:e,panel:n,head:{avatar:i,name:l,site:f},log:p,input:d,send:h}}function w(t,e,n,a=""){const i=c("div","row "+e),o=c("div","bubble"+(a?" "+a:"")),l=c("div","txt");return l.textContent=n,o.appendChild(l),i.appendChild(o),t.appendChild(i),C(t),o}function K(t,e){const n=c("div","sys");return n.textContent=e,t.appendChild(n),C(t),n}function q(t,e){const n=c("div","row ai"),a=c("div","bubble load"),i=c("span");i.textContent=e;const o=c("span","dots");return o.append(c("span"),c("span"),c("span")),a.append(i,o),n.appendChild(a),t.appendChild(n),C(t),{set:l=>i.textContent=l,remove:()=>n.remove()}}function V(t){var o,l,f,u,p;const e=c("div","datacard k-"+ee(t.kind));if(t.title){const r=c("div","dc-title");r.textContent=t.title,e.appendChild(r)}for(const r of(o=t.fields)!=null?o:[]){const d=c("div","dc-row"),h=c("span");h.textContent=r.label;const v=c("b");v.textContent=r.display,d.append(h,v),e.appendChild(d)}if(t.table&&((l=t.table.rows)!=null&&l.length)){const r=c("div","dc-tablewrap"),d=c("table","dc-table"),h=c("thead"),v=c("tr");for(const m of(f=t.table.columns)!=null?f:[]){const g=c("th");g.textContent=m.label,v.appendChild(g)}h.appendChild(v);const b=c("tbody");for(const m of t.table.rows){const g=c("tr");for(const x of m){const F=c("td");F.textContent=(u=x==null?void 0:x.display)!=null?u:"",g.appendChild(F)}b.appendChild(g)}d.append(h,b),r.appendChild(d),e.appendChild(r)}if(t.note){const r=c("div","dc-note");r.textContent=t.note,e.appendChild(r)}const n=c("div","src"),a=c("span");a.textContent=t.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+Y(t.fetched_at)+(t.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),n.appendChild(a);const i=X((p=t.link)==null?void 0:p.path);if(t.link&&i){const r=c("a");r.href=i,r.textContent=(t.link.label||"เปิดหน้าจริง")+" →",r.target="_self",r.rel="noopener",n.appendChild(r)}return e.appendChild(n),e}function Y(t){const e=new Date(t);if(isNaN(e.getTime()))return"-";const n={timeZone:"Asia/Bangkok"};try{const a=e.toLocaleTimeString("th-TH",{...n,hour:"2-digit",minute:"2-digit",hour12:!1}),i=e.toLocaleDateString("en-CA",n),o=new Date().toLocaleDateString("en-CA",n);if(i===o)return a;const[,l,f]=i.split("-");return`${f}/${l} ${a}`}catch{return e.toISOString().slice(0,16).replace("T"," ")}}function X(t){if(!t)return"";const e=t.trim();return e.startsWith("#")?e:!e.startsWith("/")||e.startsWith("//")||e.includes("\\")?"":e}function Q(t,e){const{position:n,offset_x:a,offset_y:i}=e.placement,o=n==="bottom-left";for(const l of[t.launcher,t.panel])l.style.left="",l.style.right="";t.launcher.style.bottom=k(i),t.panel.style.bottom=k(i+68),o?(t.launcher.style.left=k(a),t.panel.style.left=k(a)):(t.launcher.style.right=k(a),t.panel.style.right=k(a))}function Z(t,e){t.head.name.textContent=e.display_name||"ผู้ช่วยหลังบ้าน",t.head.site.textContent=e.service_label||e.service_id||"",t.head.avatar.textContent="",t.launcher.textContent="",e.avatar_url?(t.head.avatar.appendChild(R(e.avatar_url)),t.launcher.appendChild(R(e.avatar_url))):(t.head.avatar.textContent="AI",t.launcher.textContent="AI")}function C(t){t.scrollTop=t.scrollHeight}function R(t){const e=document.createElement("img");return e.src=t,e.alt="",e.referrerPolicy="no-referrer",e}function ee(t){return String(t||"").replace(/[^a-z_]/g,"")}function k(t){return`${t|0}px`}function c(t,e){const n=document.createElement(t);return e&&(n.className=e),n}const te=["avatar_url","display_name","greeting","theme","placement","service_label","is_hidden"],ne=2*1024*1024;function ae(t,e,n){if(!t||typeof e!="string"||!e.startsWith("/")||e.startsWith("//")||e.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(e)||e.includes("://"))return null;let a;try{a=new URL(t)}catch{return null}const i=a.pathname.replace(/\/+$/,"");let o;try{o=new URL(a.origin+i+e)}catch{return null}if(o.origin!==a.origin||!(o.pathname===i||o.pathname.startsWith(i+"/")))return null;if(n)for(const[l,f]of Object.entries(n))o.searchParams.set(l,String(f));return o.toString()}async function N(t,e,n){var f;if(!n)return{status:401,body:""};const a=String(e.method||"").toUpperCase();if(a!=="GET"&&a!=="POST")return{status:0,body:""};const i=ae(t,e.path,e.query);if(!i)return{status:0,body:""};const o=new AbortController,l=setTimeout(()=>o.abort(),Math.max(1e3,(f=e.timeout_ms)!=null?f:2e4));try{const u={method:a,credentials:"omit",signal:o.signal,headers:{Authorization:`Bearer ${n}`,Accept:"application/json"}};a==="POST"&&e.body!==void 0&&e.body!==null&&(u.headers["Content-Type"]="application/json",u.body=JSON.stringify(e.body));const p=await fetch(i,u),r=await p.text();return r.length>ne?{status:0,body:""}:{status:p.status,body:r}}catch{return{status:0,body:""}}finally{clearTimeout(l)}}const oe=180*1e3;class ie{constructor(e,n){this.apiBase=e,this.readToken=n,this.cfg=null,this.cur=null}hostApiBase(){var e;return((e=this.cfg)==null?void 0:e.host_api_base)||""}async pageConfig(){var a;if(this.cfg)return this.cfg;const e=await fetch(`${this.apiBase}/api/ai/widget/page-config`),n=await e.json().catch(()=>null);if(!e.ok||!(n!=null&&n.payload))throw new Error(`โหลดการตั้งค่าไม่สำเร็จ (${(a=n==null?void 0:n.message)!=null?a:e.status})`);return this.cfg=n.payload,this.cfg}async ticket(e){var l;if(this.cur&&this.cur.service===e&&this.cur.expiresAt-Date.now()>oe)return this.cur.ticket;const n=await this.pageConfig(),a=await this.readPermissions(n,e),i=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(e)}/browser-session`,{method:"POST",headers:{Authorization:`Bearer ${this.readToken()}`,"Content-Type":"application/json"},body:JSON.stringify({permissions:a})}),o=await i.json().catch(()=>null);if(!i.ok||!((l=o==null?void 0:o.payload)!=null&&l.ticket))throw new Error((o==null?void 0:o.error)||`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${i.status})`);return this.cur={service:e,ticket:o.payload.ticket,expiresAt:Date.now()+o.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(e,n){var l,f;const a=(l=e.identity)==null?void 0:l.permissions_request,i=(f=e.identity)==null?void 0:f.permissions;if(!a||!(i!=null&&i.path))return[];const o=await N(this.hostApiBase(),{id:"perm",method:"GET",path:a.split("{service}").join(encodeURIComponent(n))},this.readToken());if(o.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${o.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{const u=I(JSON.parse(o.body),i.path);if(!Array.isArray(u))return[];const p=[];for(const r of u){if(i.where_field&&I(r,i.where_field)!==i.where_value)continue;const d=i.pluck?I(r,i.pluck):r;typeof d=="string"&&d&&p.push(d)}return p}catch{return[]}}}function I(t,e){let n=t;for(const a of e.split(".")){if(!n||typeof n!="object")return;n=n[a]}return n}class D extends Error{}async function re(t){const{apiBase:e,service:n,session:a,on:i}=t,o=await a.ticket(n),l=`${e}/api/ai/widget/service/${encodeURIComponent(n)}`;let f=t.conversationID;const u=await fetch(`${l}/chat`,{method:"POST",headers:{Authorization:`Bearer ${o}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:f,text:t.text})});if(!u.ok||!u.body){u.status===401&&a.invalidate();const h=await u.json().catch(()=>null);throw new D((h==null?void 0:h.error)||`เซิร์ฟเวอร์ตอบ ${u.status}`)}const p=[],r=h=>p.push(N(a.hostApiBase(),h,t.readToken()).then(v=>fetch(`${l}/chat/relay/${encodeURIComponent(h.id)}`,{method:"POST",headers:{Authorization:`Bearer ${o}`,"Content-Type":"application/json"},body:JSON.stringify(v)})).then(()=>{}).catch(()=>{}));let d=null;if(await se(u.body,(h,v)=>{var m,g,x;const b=v;switch(h){case"status":typeof b.conversation_id=="string"&&(f=b.conversation_id),i.status(String((m=b.text)!=null?m:""));break;case"fetch":r(b);break;case"card":i.card(b);break;case"token":i.token(String((g=b.text)!=null?g:""));break;case"error":d=new D(String((x=b.message)!=null?x:"ผู้ช่วยตอบไม่สำเร็จ"));break}}),await Promise.all(p),d)throw d;return f}async function se(t,e){const n=t.getReader(),a=new TextDecoder;let i="";for(;;){const{value:o,done:l}=await n.read();if(l)break;i+=a.decode(o,{stream:!0});let f;for(;(f=i.indexOf(`

`))>=0;){const u=i.slice(0,f);i=i.slice(f+2);let p="message",r="";for(const d of u.split(`
`))d.startsWith("event:")?p=d.slice(6).trim():d.startsWith("data:")&&(r+=d.slice(5).trim());r&&e(p,JSON.parse(r))}}}const ce=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],de="auth_token",le="web-service";function _(){try{const t=localStorage.getItem(de);if(!t)return"";const e=JSON.parse(t);if(!(e!=null&&e.value))return"";const n=Math.floor(Date.now()/1e3);return typeof e.expiration=="number"&&e.expiration<=n?"":e.value}catch{return""}}function S(){var t;try{return(t=localStorage.getItem(le))!=null?t:""}catch{return""}}const O={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}};let s=null;function B(){window.removeEventListener("message",M),s==null||s.host.remove(),s=null}async function $(t={}){var d,h,v,b,m;B();const e=(d=t.dataset)!=null?d:{};for(const g of ce)e[g]&&(console.warn(`[ai-office] ไม่รับ data-${ue(g)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete e[g]);const n=(h=e.previewMount)!=null?h:"",a=n!=="",i=(b=(v=t.apiBase)!=null?v:e.apiBase)!=null?b:"";let o;if(a)o={...O,...(m=t.bootstrap)!=null?m:{}};else if(t.bootstrap)o={...O,...t.bootstrap};else{const g=await pe(i,t.onFetch);if(!g)return;o={...O,...g}}if(!a&&!o.enabled)return;const l=a?document.querySelector(n):null;if(a&&!l){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:f,root:u}=H(l);a&&f.setAttribute("data-preview","true"),j(u);const p=J(u),r=new ie(i,_);s={ui:p,host:f,cfg:o,preview:a,apiBase:i,session:r,conversationID:"",busy:!1},P(o),p.launcher.addEventListener("click",()=>L()),p.send.addEventListener("click",()=>z(t)),p.input.addEventListener("keydown",g=>{const x=g;x.key==="Enter"&&!x.shiftKey&&!x.isComposing&&(x.preventDefault(),z(t))}),a&&(p.panel.dataset.open="true",window.addEventListener("message",M)),he()}function P(t){s&&(s.cfg=t,s.host.setAttribute("data-theme",fe(t.theme)),Z(s.ui,t),Q(s.ui,t),s.ui.launcher.style.display=t.is_hidden?"none":"grid",s.ui.log.textContent="",t.greeting&&w(s.ui.log,"ai",t.greeting),s.preview&&K(s.ui.log,T.preview))}function M(t){if(t.origin!==window.location.origin)return;const e=t.data;if(!e||e.type!=="ai-office:preview-config"||!e.config||!s||!s.preview)return;const n={};for(const a of te)a in e.config&&(n[a]=e.config[a]);P({...s.cfg,...n})}async function pe(t,e){var o,l,f,u,p;const n=_(),a=S();if(!n||!a)return A(!n&&!a?'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน':n?'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน':'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'),null;const i=`${t}/api/ai/widget/service/${encodeURIComponent(a)}/bootstrap`;e==null||e({url:i});try{const r=await fetch(i,{headers:{Authorization:`Bearer ${n}`}}),d=await r.json().catch(()=>null);if(!r.ok)return A(`เซิร์ฟเวอร์ตอบ ${r.status} ${(o=d==null?void 0:d.message)!=null?o:""} — ${(l=d==null?void 0:d.error)!=null?l:""}`.trim(),U[(f=d==null?void 0:d.message)!=null?f:""]),null;const h=(u=d==null?void 0:d.payload)!=null?u:null;return h&&!h.enabled&&A(`ยังไม่เปิดใช้งาน (reason: ${h.reason})`,U[(p=h.reason)!=null?p:""]),h}catch(r){return A(`เรียก ${i} ไม่สำเร็จ — ${r.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function A(t,e){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${t}`+(e?`
           → ${e}`:""))}const U={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "โดเมนที่อนุญาต" ของ office ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน",SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function L(t){if(!s)return;const e=t!=null?t:s.ui.panel.dataset.open!=="true";s.ui.panel.dataset.open=String(e),e&&s.ui.input.focus()}const T={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)"};async function z(t){var u;const e=s;if(!e||e.busy||e.ui.input.disabled)return;const n=e.ui.input.value.trim();if(!n)return;if(e.ui.input.value="",w(e.ui.log,"me",n),e.preview){w(e.ui.log,"ai",T.preview);return}const a=S();(u=t.onFetch)==null||u.call(t,{url:`${e.apiBase}/api/ai/widget/service/${encodeURIComponent(a)}/chat`,body:{text:n}}),e.busy=!0,e.ui.send.disabled=!0;const i=q(e.ui.log,"กำลังส่งคำถาม…");let o=null,l=null;const f=()=>{if(o)return;const p=w(e.ui.log,"ai","");o=p.querySelector(".txt"),l=c("div","cards"),p.appendChild(l)};try{e.conversationID=await re({apiBase:e.apiBase,service:a,session:e.session,readToken:_,conversationID:e.conversationID,text:n,on:{status:p=>i.set(p||"กำลังทำงาน…"),card:p=>{f(),l.appendChild(V(p)),C(e.ui.log)},token:p=>{var r;i.remove(),f(),o.textContent=((r=o.textContent)!=null?r:"")+p,C(e.ui.log)}}}),o||w(e.ui.log,"ai",T.empty)}catch(p){const r=p instanceof D?p.message:T.unavailable;w(e.ui.log,"ai",r,"err")}finally{i.remove(),e.busy=!1,e.ui.send.disabled=!1}}function fe(t){var e,n;return t==="light"||t==="dark"?t:(n=(e=window.matchMedia)==null?void 0:e.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function ue(t){return t.replace(/[A-Z]/g,e=>"-"+e.toLowerCase())}function he(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>L(!0),close:()=>L(!1),__hasLauncher:()=>!!s&&s.ui.launcher.style.display!=="none",__isOpen:()=>!!s&&s.ui.panel.dataset.open==="true",__launcherStyle:()=>s?s.ui.launcher.style:{},__config:()=>s?{...s.cfg}:{},__host:()=>{var e;return(e=s==null?void 0:s.host)!=null?e:null},__send:async e=>{s&&(s.ui.input.value=e,await z({}))},__logText:()=>{var e;return(e=s==null?void 0:s.ui.log.textContent)!=null?e:""},__conversationID:()=>{var e;return(e=s==null?void 0:s.conversationID)!=null?e:""}}),configurable:!0})}const E=document.currentScript;if(E){const t={};for(const o in E.dataset)t[o]=E.dataset[o];E.src&&(t.apiBase||(t.apiBase=new URL(E.src,location.href).origin));let e=null;const n=()=>_()?S()||"\0":"",a=()=>{try{const o=n();if(o===e)return;e=o,o===""?B():$({dataset:t})}catch{}},i=()=>{if(t.previewMount){$({dataset:t});return}a(),setInterval(a,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",i):i()}return y.mount=$,y.readOfficeService=S,y.readOfficeToken=_,y.unmount=B,Object.defineProperty(y,Symbol.toStringTag,{value:"Module"}),y})({});
