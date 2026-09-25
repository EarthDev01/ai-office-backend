var AIOffice=(function(k){"use strict";const Y=`
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
`;function V(t){const e=document.createElement("div");e.setAttribute("data-ai-office-host",""),e.style.all="initial",t?(getComputedStyle(t).position==="static"&&(t.style.position="relative"),e.style.position="absolute",e.style.inset="0",t.appendChild(e)):document.body.appendChild(e);const n=e.attachShadow({mode:"closed"}),a=document.createElement("style");return a.textContent=Y,n.appendChild(a),{host:e,root:n}}function X(t){const e=document.createElement("link");e.rel="stylesheet",e.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",t.appendChild(e)}const Q=2e3;function Z(t){const e=d("button","launcher");e.type="button",e.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน");const n=d("div","panel");n.dataset.open="false",n.setAttribute("role","dialog"),n.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const a=d("div","head"),i=d("div","avatar"),o=d("div","namewrap"),s=d("div","name");o.appendChild(s);const f=d("div","site");f.title="เว็บที่กำลังคุยอยู่";const p=d("button","x");p.type="button",p.textContent="×",p.setAttribute("aria-label","ปิด"),a.append(i,o,f,p);const u=d("div","log");u.setAttribute("aria-live","polite");const r=d("div","foot"),c=d("textarea");c.rows=1,c.maxLength=Q,c.placeholder="พิมพ์คำถาม…",c.setAttribute("aria-label","คำถาม");const h=d("button","send");return h.type="button",h.textContent="↑",h.setAttribute("aria-label","ส่ง"),r.append(c,h),n.append(a,u,r),t.append(e,n),p.addEventListener("click",()=>n.dataset.open="false"),{launcher:e,panel:n,head:{avatar:i,name:s,site:f},log:u,input:c,send:h}}function x(t,e,n,a=""){const i=d("div","row "+e),o=d("div","bubble"+(a?" "+a:"")),s=d("div","txt");return s.textContent=n,o.appendChild(s),i.appendChild(o),t.appendChild(i),C(t),o}function ee(t,e){const n=d("div","sys");return n.textContent=e,t.appendChild(n),C(t),n}function M(t,e){const n=d("div","row ai"),a=d("div","bubble load"),i=d("span");i.textContent=e;const o=d("span","dots");return o.append(d("span"),d("span"),d("span")),a.append(i,o),n.appendChild(a),t.appendChild(n),C(t),{set:s=>i.textContent=s,remove:()=>n.remove()}}function U(t){var o,s,f,p,u;const e=d("div","datacard k-"+ie(t.kind));if(t.title){const r=d("div","dc-title");r.textContent=t.title,e.appendChild(r)}for(const r of(o=t.fields)!=null?o:[]){const c=d("div","dc-row"),h=d("span");h.textContent=r.label;const b=d("b");b.textContent=r.display,c.append(h,b),e.appendChild(c)}if(t.table&&((s=t.table.rows)!=null&&s.length)){const r=d("div","dc-tablewrap"),c=d("table","dc-table"),h=d("thead"),b=d("tr");for(const v of(f=t.table.columns)!=null?f:[]){const m=d("th");m.textContent=v.label,b.appendChild(m)}h.appendChild(b);const g=d("tbody");for(const v of t.table.rows){const m=d("tr");for(const y of v){const I=d("td");I.textContent=(p=y==null?void 0:y.display)!=null?p:"",m.appendChild(I)}g.appendChild(m)}c.append(h,g),r.appendChild(c),e.appendChild(r)}if(t.note){const r=d("div","dc-note");r.textContent=t.note,e.appendChild(r)}const n=d("div","src"),a=d("span");a.textContent=t.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+te(t.fetched_at)+(t.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),n.appendChild(a);const i=ne((u=t.link)==null?void 0:u.path);if(t.link&&i){const r=d("a");r.href=i,r.textContent=(t.link.label||"เปิดหน้าจริง")+" →",r.target="_self",r.rel="noopener",n.appendChild(r)}return e.appendChild(n),e}function te(t){const e=new Date(t);if(isNaN(e.getTime()))return"-";const n={timeZone:"Asia/Bangkok"};try{const a=e.toLocaleTimeString("th-TH",{...n,hour:"2-digit",minute:"2-digit",hour12:!1}),i=e.toLocaleDateString("en-CA",n),o=new Date().toLocaleDateString("en-CA",n);if(i===o)return a;const[,s,f]=i.split("-");return`${f}/${s} ${a}`}catch{return e.toISOString().slice(0,16).replace("T"," ")}}function ne(t){if(!t)return"";const e=t.trim();return e.startsWith("#")?e:!e.startsWith("/")||e.startsWith("//")||e.includes("\\")?"":e}function ae(t,e){const{position:n,offset_x:a,offset_y:i}=e.placement,o=n==="bottom-left";for(const s of[t.launcher,t.panel])s.style.left="",s.style.right="";t.launcher.style.bottom=_(i),t.panel.style.bottom=_(i+68),o?(t.launcher.style.left=_(a),t.panel.style.left=_(a)):(t.launcher.style.right=_(a),t.panel.style.right=_(a))}function oe(t,e){t.head.name.textContent=e.display_name||"ผู้ช่วยหลังบ้าน",t.head.site.textContent=e.service_label||e.service_id||"",t.head.avatar.textContent="",t.launcher.textContent="",e.avatar_url?(t.head.avatar.appendChild(F(e.avatar_url)),t.launcher.appendChild(F(e.avatar_url))):(t.head.avatar.textContent="AI",t.launcher.textContent="AI")}function C(t){t.scrollTop=t.scrollHeight}function F(t){const e=document.createElement("img");return e.src=t,e.alt="",e.referrerPolicy="no-referrer",e}function ie(t){return String(t||"").replace(/[^a-z_]/g,"")}function _(t){return`${t|0}px`}function d(t,e){const n=document.createElement(t);return e&&(n.className=e),n}const re=["avatar_url","display_name","greeting","theme","placement","service_label","is_hidden"],se=2*1024*1024;function le(t,e,n){if(!t||typeof e!="string"||!e.startsWith("/")||e.startsWith("//")||e.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(e)||e.includes("://"))return null;let a;try{a=new URL(t)}catch{return null}const i=a.pathname.replace(/\/+$/,"");let o;try{o=new URL(a.origin+i+e)}catch{return null}if(o.origin!==a.origin||!(o.pathname===i||o.pathname.startsWith(i+"/")))return null;if(n)for(const[s,f]of Object.entries(n))o.searchParams.set(s,String(f));return o.toString()}async function W(t,e,n){var f;if(!n)return{status:401,body:""};const a=String(e.method||"").toUpperCase();if(a!=="GET"&&a!=="POST")return{status:0,body:""};const i=le(t,e.path,e.query);if(!i)return{status:0,body:""};const o=new AbortController,s=setTimeout(()=>o.abort(),Math.max(1e3,(f=e.timeout_ms)!=null?f:2e4));try{const p={method:a,credentials:"omit",signal:o.signal,headers:{Authorization:`Bearer ${n}`,Accept:"application/json"}};a==="POST"&&e.body!==void 0&&e.body!==null&&(p.headers["Content-Type"]="application/json",p.body=JSON.stringify(e.body));const u=await fetch(i,p),r=await u.text();return r.length>se?{status:0,body:""}:{status:u.status,body:r}}catch{return{status:0,body:""}}finally{clearTimeout(s)}}const ce=180*1e3;class de{constructor(e,n){this.apiBase=e,this.readToken=n,this.cfg=null,this.cur=null}hostApiBase(){var e;return((e=this.cfg)==null?void 0:e.host_api_base)||""}async pageConfig(){var a;if(this.cfg)return this.cfg;const e=await fetch(`${this.apiBase}/api/ai/widget/page-config`),n=await e.json().catch(()=>null);if(!e.ok||!(n!=null&&n.payload))throw new Error(`โหลดการตั้งค่าไม่สำเร็จ (${(a=n==null?void 0:n.message)!=null?a:e.status})`);return this.cfg=n.payload,this.cfg}async ticket(e){var s;if(this.cur&&this.cur.service===e&&this.cur.expiresAt-Date.now()>ce)return this.cur.ticket;const n=await this.pageConfig(),a=await this.readPermissions(n,e),i=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(e)}/browser-session`,{method:"POST",headers:{Authorization:`Bearer ${this.readToken()}`,"Content-Type":"application/json"},body:JSON.stringify({permissions:a})}),o=await i.json().catch(()=>null);if(!i.ok||!((s=o==null?void 0:o.payload)!=null&&s.ticket))throw new Error((o==null?void 0:o.error)||`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${i.status})`);return this.cur={service:e,ticket:o.payload.ticket,expiresAt:Date.now()+o.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(e,n){var s,f;const a=(s=e.identity)==null?void 0:s.permissions_request,i=(f=e.identity)==null?void 0:f.permissions;if(!a||!(i!=null&&i.path))return[];const o=await W(this.hostApiBase(),{id:"perm",method:"GET",path:a.split("{service}").join(encodeURIComponent(n))},this.readToken());if(o.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${o.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{const p=P(JSON.parse(o.body),i.path);if(!Array.isArray(p))return[];const u=[];for(const r of p){if(i.where_field&&P(r,i.where_field)!==i.where_value)continue;const c=i.pluck?P(r,i.pluck):r;typeof c=="string"&&c&&u.push(c)}return u}catch{return[]}}}function P(t,e){let n=t;for(const a of e.split(".")){if(!n||typeof n!="object")return;n=n[a]}return n}class S extends Error{}async function ue(t){const{apiBase:e,service:n,session:a,on:i}=t,o=await a.ticket(n),s=`${e}/api/ai/widget/service/${encodeURIComponent(n)}`;let f=t.conversationID;const p=await fetch(`${s}/chat`,{method:"POST",headers:{Authorization:`Bearer ${o}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:f,text:t.text})});if(!p.ok||!p.body){p.status===401&&a.invalidate();const h=await p.json().catch(()=>null);throw new S((h==null?void 0:h.error)||`เซิร์ฟเวอร์ตอบ ${p.status}`)}const u=[],r=h=>u.push(W(a.hostApiBase(),h,t.readToken()).then(b=>fetch(`${s}/chat/relay/${encodeURIComponent(h.id)}`,{method:"POST",headers:{Authorization:`Bearer ${o}`,"Content-Type":"application/json"},body:JSON.stringify(b)})).then(()=>{}).catch(()=>{}));let c=null;if(await j(p.body,(h,b)=>{var v,m,y;const g=b;switch(h){case"status":typeof g.conversation_id=="string"&&(f=g.conversation_id),i.status(String((v=g.text)!=null?v:""));break;case"fetch":r(g);break;case"card":i.card(g);break;case"token":i.token(String((m=g.text)!=null?m:""));break;case"error":c=new S(String((y=g.message)!=null?y:"ผู้ช่วยตอบไม่สำเร็จ"));break}}),await Promise.all(u),c)throw c;return f}async function fe(t){const e=await fetch(`${t.apiBase}/api/ai/admin/assistant`,{method:"POST",headers:{Authorization:`Bearer ${t.token}`,"Content-Type":"application/json"},body:JSON.stringify({messages:t.messages,page:t.page})});if(!e.ok||!e.body){const a=await e.json().catch(()=>null);throw new S((a==null?void 0:a.error)||`เซิร์ฟเวอร์ตอบ ${e.status}`)}let n=null;if(await j(e.body,(a,i)=>{var s,f,p;const o=i;a==="status"?t.on.status(String((s=o.text)!=null?s:"")):a==="card"?t.on.card(o):a==="token"?t.on.token(String((f=o.text)!=null?f:"")):a==="error"&&(n=new S(String((p=o.message)!=null?p:"ผู้ช่วยตอบไม่สำเร็จ")))}),n)throw n}async function j(t,e){const n=t.getReader(),a=new TextDecoder;let i="";for(;;){const{value:o,done:s}=await n.read();if(s)break;i+=a.decode(o,{stream:!0});let f;for(;(f=i.indexOf(`

`))>=0;){const p=i.slice(0,f);i=i.slice(f+2);let u="message",r="";for(const c of p.split(`
`))c.startsWith("event:")?u=c.slice(6).trim():c.startsWith("data:")&&(r+=c.slice(5).trim());r&&e(u,JSON.parse(r))}}}const pe=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],he="auth_token",ge="web-service";function A(){try{const t=localStorage.getItem(he);if(!t)return"";const e=JSON.parse(t);if(!(e!=null&&e.value))return"";const n=Math.floor(Date.now()/1e3);return typeof e.expiration=="number"&&e.expiration<=n?"":e.value}catch{return""}}function B(){var t;try{return(t=localStorage.getItem(ge))!=null?t:""}catch{return""}}const D={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}},be=20;let l=null;function $(){window.removeEventListener("message",H),l==null||l.host.remove(),l=null}async function L(t={}){var c,h,b,g,v,m,y,I,q;$();const e=(c=t.dataset)!=null?c:{};for(const w of pe)e[w]&&(console.warn(`[ai-office] ไม่รับ data-${ye(w)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete e[w]);const n=(h=e.previewMount)!=null?h:"",a=n!=="",i=(g=(b=t.apiBase)!=null?b:e.apiBase)!=null?g:"";let o;if(t.console)o={...D,display_name:(v=t.console.displayName)!=null?v:"ผู้ช่วย AI Office",greeting:(m=t.console.greeting)!=null?m:"สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)",service_label:(y=t.console.label)!=null?y:"คอนโซล",placement:{position:"bottom-right",offset_x:20,offset_y:20}};else if(a)o={...D,...(I=t.bootstrap)!=null?I:{}};else if(t.bootstrap)o={...D,...t.bootstrap};else{const w=await me(i,t.onFetch);if(!w)return;o={...D,...w}}if(!a&&!o.enabled)return;const s=a?document.querySelector(n):null;if(a&&!s){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:f,root:p}=V(s);a&&f.setAttribute("data-preview","true"),X(p);const u=Z(p),r=new de(i,A);l={ui:u,host:f,cfg:o,preview:a,apiBase:i,session:r,conversationID:"",busy:!1,console:(q=t.console)!=null?q:null,history:[]},G(o),u.launcher.addEventListener("click",()=>T()),u.send.addEventListener("click",()=>R(t)),u.input.addEventListener("keydown",w=>{const N=w;N.key==="Enter"&&!N.shiftKey&&!N.isComposing&&(N.preventDefault(),R(t))}),a&&(u.panel.dataset.open="true",window.addEventListener("message",H)),t.console||we()}function G(t){l&&(l.cfg=t,l.host.setAttribute("data-theme",ve(t.theme)),oe(l.ui,t),ae(l.ui,t),l.ui.launcher.style.display=t.is_hidden?"none":"grid",l.ui.log.textContent="",t.greeting&&x(l.ui.log,"ai",t.greeting),l.preview&&ee(l.ui.log,E.preview))}function H(t){if(t.origin!==window.location.origin)return;const e=t.data;if(!e||e.type!=="ai-office:preview-config"||!e.config||!l||!l.preview)return;const n={};for(const a of re)a in e.config&&(n[a]=e.config[a]);G({...l.cfg,...n})}async function me(t,e){var o,s,f,p,u;const n=A(),a=B();if(!n||!a)return z(!n&&!a?'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน':n?'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน':'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'),null;const i=`${t}/api/ai/widget/service/${encodeURIComponent(a)}/bootstrap`;e==null||e({url:i});try{const r=await fetch(i,{headers:{Authorization:`Bearer ${n}`}}),c=await r.json().catch(()=>null);if(!r.ok)return z(`เซิร์ฟเวอร์ตอบ ${r.status} ${(o=c==null?void 0:c.message)!=null?o:""} — ${(s=c==null?void 0:c.error)!=null?s:""}`.trim(),J[(f=c==null?void 0:c.message)!=null?f:""]),null;const h=(p=c==null?void 0:c.payload)!=null?p:null;return h&&!h.enabled&&z(`ยังไม่เปิดใช้งาน (reason: ${h.reason})`,J[(u=h.reason)!=null?u:""]),h}catch(r){return z(`เรียก ${i} ไม่สำเร็จ — ${r.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function z(t,e){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${t}`+(e?`
           → ${e}`:""))}const J={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน",SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function T(t){if(!l)return;const e=t!=null?t:l.ui.panel.dataset.open!=="true";l.ui.panel.dataset.open=String(e),e&&l.ui.input.focus()}const E={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)"};async function R(t){var p;const e=l;if(!e||e.busy||e.ui.input.disabled)return;const n=e.ui.input.value.trim();if(!n)return;if(e.ui.input.value="",x(e.ui.log,"me",n),e.preview){x(e.ui.log,"ai",E.preview);return}if(e.console){await xe(e,e.console,n);return}const a=B();(p=t.onFetch)==null||p.call(t,{url:`${e.apiBase}/api/ai/widget/service/${encodeURIComponent(a)}/chat`,body:{text:n}}),e.busy=!0,e.ui.send.disabled=!0;const i=M(e.ui.log,"กำลังส่งคำถาม…");let o=null,s=null;const f=()=>{if(o)return;const u=x(e.ui.log,"ai","");o=u.querySelector(".txt"),s=d("div","cards"),u.appendChild(s)};try{e.conversationID=await ue({apiBase:e.apiBase,service:a,session:e.session,readToken:A,conversationID:e.conversationID,text:n,on:{status:u=>i.set(u||"กำลังทำงาน…"),card:u=>{f(),s.appendChild(U(u)),C(e.ui.log)},token:u=>{var r;i.remove(),f(),o.textContent=((r=o.textContent)!=null?r:"")+u,C(e.ui.log)}}}),o||x(e.ui.log,"ai",E.empty)}catch(u){const r=u instanceof S?u.message:E.unavailable;x(e.ui.log,"ai",r,"err")}finally{i.remove(),e.busy=!1,e.ui.send.disabled=!1}}async function xe(t,e,n){var p,u;t.history.push({role:"user",text:n}),t.busy=!0,t.ui.send.disabled=!0;const a=M(t.ui.log,"กำลังส่งคำถาม…");let i=null,o=null,s="";const f=()=>{if(i)return;const r=x(t.ui.log,"ai","");i=r.querySelector(".txt"),o=d("div","cards"),r.appendChild(o)};try{await fe({apiBase:t.apiBase,token:e.getToken(),messages:t.history.slice(-be),page:(u=(p=e.getPage)==null?void 0:p.call(e))!=null?u:"",on:{status:r=>a.set(r||"กำลังทำงาน…"),card:r=>{f(),o.appendChild(U(r)),C(t.ui.log)},token:r=>{a.remove(),f(),s+=r,i.textContent=s,C(t.ui.log)}}}),s?t.history.push({role:"assistant",text:s}):x(t.ui.log,"ai",E.empty)}catch(r){t.history.pop();const c=r instanceof S?r.message:E.unavailable;x(t.ui.log,"ai",c,"err")}finally{a.remove(),t.busy=!1,t.ui.send.disabled=!1}}function ve(t){var e,n;return t==="light"||t==="dark"?t:(n=(e=window.matchMedia)==null?void 0:e.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function ye(t){return t.replace(/[A-Z]/g,e=>"-"+e.toLowerCase())}function K(t){Object.defineProperty(window,"__aiOfficeConsole",{value:Object.freeze({mount:n=>L({apiBase:t,console:n}),unmount:$,open:()=>T(!0),close:()=>T(!1),__logText:()=>{var n;return(n=l==null?void 0:l.ui.log.textContent)!=null?n:""},__send:async n=>{l&&(l.ui.input.value=n,await R({}))}}),configurable:!0})}function we(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>T(!0),close:()=>T(!1),__hasLauncher:()=>!!l&&l.ui.launcher.style.display!=="none",__isOpen:()=>!!l&&l.ui.panel.dataset.open==="true",__launcherStyle:()=>l?l.ui.launcher.style:{},__config:()=>l?{...l.cfg}:{},__host:()=>{var e;return(e=l==null?void 0:l.host)!=null?e:null},__send:async e=>{l&&(l.ui.input.value=e,await R({}))},__logText:()=>{var e;return(e=l==null?void 0:l.ui.log.textContent)!=null?e:""},__conversationID:()=>{var e;return(e=l==null?void 0:l.conversationID)!=null?e:""}}),configurable:!0})}const O=document.currentScript;if(O){const t={};for(const o in O.dataset)t[o]=O.dataset[o];O.src&&(t.apiBase||(t.apiBase=new URL(O.src,location.href).origin));let e=null;const n=()=>A()?B()||"\0":"",a=()=>{try{const o=n();if(o===e)return;e=o,o===""?$():L({dataset:t})}catch{}},i=()=>{var o;if(t.consoleMode!==void 0){K((o=t.apiBase)!=null?o:"");return}if(t.previewMount){L({dataset:t});return}a(),setInterval(a,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",i):i()}return k.installConsoleGlobal=K,k.mount=L,k.readOfficeService=B,k.readOfficeToken=A,k.unmount=$,Object.defineProperty(k,Symbol.toStringTag,{value:"Module"}),k})({});
