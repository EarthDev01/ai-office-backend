var AIOffice=(function(v){"use strict";const $=`
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
`;function N(t){const e=document.createElement("div");e.setAttribute("data-ai-office-host",""),e.style.all="initial",t?(getComputedStyle(t).position==="static"&&(t.style.position="relative"),e.style.position="absolute",e.style.inset="0",t.appendChild(e)):document.body.appendChild(e);const n=e.attachShadow({mode:"closed"}),o=document.createElement("style");return o.textContent=$,n.appendChild(o),{host:e,root:n}}function R(t){const e=document.createElement("link");e.rel="stylesheet",e.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",t.appendChild(e)}function z(t){const e=u("button","launcher");e.type="button",e.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน");const n=u("div","panel");n.dataset.open="false";const o=u("div","head"),i=u("div","avatar"),s=u("div"),d=u("div","name");s.appendChild(d);const p=u("div","site"),l=u("button","x");l.type="button",l.textContent="×",l.setAttribute("aria-label","ปิด"),o.append(i,s,p,l);const c=u("div","log"),f=u("div","foot"),r=u("input");r.type="text",r.placeholder="พิมพ์คำถาม…";const h=u("button");return h.type="button",h.textContent="↑",f.append(r,h),n.append(o,c,f),t.append(e,n),l.addEventListener("click",()=>n.dataset.open="false"),{launcher:e,panel:n,head:{avatar:i,name:d,site:p},log:c,input:r,send:h}}function m(t,e,n){const o=u("div","row "+e),i=u("div","bubble");return i.textContent=n,o.appendChild(i),t.appendChild(o),t.scrollTop=t.scrollHeight,i}function M(t,e){const n=u("div","note");n.textContent=e,t.appendChild(n),t.scrollTop=t.scrollHeight}function P(t,e){const{position:n,offset_x:o,offset_y:i}=e.placement,s=n==="bottom-left";for(const d of[t.launcher,t.panel])d.style.left="",d.style.right="";t.launcher.style.bottom=b(i),t.panel.style.bottom=b(i+68),s?(t.launcher.style.left=b(o),t.panel.style.left=b(o)):(t.launcher.style.right=b(o),t.panel.style.right=b(o))}function F(t,e){t.head.name.textContent=e.display_name||"ผู้ช่วยหลังบ้าน",t.head.site.textContent=e.service_label||e.service_id||"",t.head.avatar.textContent="",t.launcher.textContent="",e.avatar_url?(t.head.avatar.appendChild(A(e.avatar_url)),t.launcher.appendChild(A(e.avatar_url))):(t.head.avatar.textContent="AI",t.launcher.textContent="AI")}function A(t){const e=document.createElement("img");return e.src=t,e.alt="",e.referrerPolicy="no-referrer",e}function b(t){return`${t|0}px`}function u(t,e){const n=document.createElement(t);return e&&(n.className=e),n}const U=["avatar_url","display_name","greeting","theme","placement","service_label","is_hidden"],W=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],H="auth_token",K="web-service";function y(){try{const t=localStorage.getItem(H);if(!t)return"";const e=JSON.parse(t);if(!(e!=null&&e.value))return"";const n=Math.floor(Date.now()/1e3);return typeof e.expiration=="number"&&e.expiration<=n?"":e.value}catch{return""}}function w(){var t;try{return(t=localStorage.getItem(K))!=null?t:""}catch{return""}}const _={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}};let a=null;function k(){window.removeEventListener("message",T),a==null||a.host.remove(),a=null}async function C(t={}){var f,r,h,D,L;k();const e=(f=t.dataset)!=null?f:{};for(const g of W)e[g]&&(console.warn(`[ai-office] ไม่รับ data-${Y(g)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete e[g]);const n=(r=e.previewMount)!=null?r:"",o=n!=="",i=(D=(h=t.apiBase)!=null?h:e.apiBase)!=null?D:"";let s;if(o)s={..._,...(L=t.bootstrap)!=null?L:{}};else if(t.bootstrap)s={..._,...t.bootstrap};else{const g=await G(i,t.onFetch);if(!g)return;s={..._,...g}}if(!o&&!s.enabled)return;const d=o?document.querySelector(n):null;if(o&&!d){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:p,root:l}=N(d);o&&p.setAttribute("data-preview","true"),R(l);const c=z(l);a={ui:c,host:p,cfg:s,preview:o,apiBase:i,history:[],busy:!1},I(s),c.launcher.addEventListener("click",()=>S()),c.send.addEventListener("click",()=>B(t)),c.input.addEventListener("keydown",g=>{g.key==="Enter"&&B(t)}),o&&(c.panel.dataset.open="true",window.addEventListener("message",T)),j()}function I(t){a&&(a.cfg=t,a.host.setAttribute("data-theme",J(t.theme)),F(a.ui,t),P(a.ui,t),a.ui.launcher.style.display=t.is_hidden?"none":"grid",a.ui.log.textContent="",t.greeting&&m(a.ui.log,"ai",t.greeting),M(a.ui.log,"ยังต่อกับข้อมูลจริงไม่ได้ — รอบนี้ทดสอบการติดตั้งและการตั้งค่าเท่านั้น"))}function T(t){if(t.origin!==window.location.origin)return;const e=t.data;if(!e||e.type!=="ai-office:preview-config"||!e.config||!a||!a.preview)return;const n={};for(const o of U)o in e.config&&(n[o]=e.config[o]);I({...a.cfg,...n})}async function G(t,e){var s,d,p,l,c;const n=y(),o=w();if(!n||!o)return E(!n&&!o?'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน':n?'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน':'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'),null;const i=`${t}/api/ai/widget/service/${encodeURIComponent(o)}/bootstrap`;e==null||e({url:i});try{const f=await fetch(i,{headers:{Authorization:`Bearer ${n}`}}),r=await f.json().catch(()=>null);if(!f.ok)return E(`เซิร์ฟเวอร์ตอบ ${f.status} ${(s=r==null?void 0:r.message)!=null?s:""} — ${(d=r==null?void 0:r.error)!=null?d:""}`.trim(),O[(p=r==null?void 0:r.message)!=null?p:""]),null;const h=(l=r==null?void 0:r.payload)!=null?l:null;return h&&!h.enabled&&E(`ยังไม่เปิดใช้งาน (reason: ${h.reason})`,O[(c=h.reason)!=null?c:""]),h}catch(f){return E(`เรียก ${i} ไม่สำเร็จ — ${f.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function E(t,e){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${t}`+(e?`
           → ${e}`:""))}const O={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "โดเมนที่อนุญาต" ของ office ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน",SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function S(t){if(!a)return;const e=t!=null?t:a.ui.panel.dataset.open!=="true";a.ui.panel.dataset.open=String(e),e&&a.ui.input.focus()}async function B(t){var p;const e=a;if(!e||e.busy)return;const n=e.ui.input.value.trim();if(!n)return;if(e.ui.input.value="",m(e.ui.log,"me",n),e.preview){m(e.ui.log,"ai","นี่คือตัวอย่างหน้าตา — แชทจริงใช้ได้ในหน้า office ที่ล็อกอินแล้ว");return}e.history.push({role:"user",text:n});const o=e.history.slice(-40),i=`${e.apiBase}/api/ai/widget/service/${encodeURIComponent(w())}/chat`;(p=t.onFetch)==null||p.call(t,{url:i,body:{messages:o}}),e.busy=!0,e.ui.send.disabled=!0;const s=m(e.ui.log,"ai","…");let d="";try{const l=await fetch(i,{method:"POST",headers:{Authorization:`Bearer ${y()}`,"Content-Type":"application/json"},body:JSON.stringify({messages:o})});if(!l.ok||!l.body){const c=await l.json().catch(()=>null);throw new Error((c==null?void 0:c.error)||`เซิร์ฟเวอร์ตอบ ${l.status}`)}await V(l.body,(c,f)=>{var r;if(c==="delta")d+=(r=f.text)!=null?r:"",s.textContent=d,e.ui.log.scrollTop=e.ui.log.scrollHeight;else if(c==="error")throw new Error(f.message||"ผู้ช่วยตอบไม่สำเร็จ")}),d?e.history.push({role:"ai",text:d}):s.textContent="(ไม่มีคำตอบ)"}catch(l){e.history.pop(),s.textContent=d||"⚠︎ "+l.message}finally{e.busy=!1,e.ui.send.disabled=!1}}async function V(t,e){const n=t.getReader(),o=new TextDecoder;let i="";for(;;){const{value:s,done:d}=await n.read();if(d)break;i+=o.decode(s,{stream:!0});let p;for(;(p=i.indexOf(`

`))>=0;){const l=i.slice(0,p);i=i.slice(p+2);let c="message",f="";for(const r of l.split(`
`))r.startsWith("event:")?c=r.slice(6).trim():r.startsWith("data:")&&(f+=r.slice(5).trim());f&&e(c,JSON.parse(f))}}}function J(t){var e,n;return t==="light"||t==="dark"?t:(n=(e=window.matchMedia)==null?void 0:e.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function Y(t){return t.replace(/[A-Z]/g,e=>"-"+e.toLowerCase())}function j(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>S(!0),close:()=>S(!1),__hasLauncher:()=>!!a&&a.ui.launcher.style.display!=="none",__isOpen:()=>!!a&&a.ui.panel.dataset.open==="true",__launcherStyle:()=>a?a.ui.launcher.style:{},__config:()=>a?{...a.cfg}:{},__host:()=>{var e;return(e=a==null?void 0:a.host)!=null?e:null}}),configurable:!0})}const x=document.currentScript;if(x){const t={};for(const s in x.dataset)t[s]=x.dataset[s];x.src&&(t.apiBase||(t.apiBase=new URL(x.src,location.href).origin));let e=null;const n=()=>y()?w()||"\0":"",o=()=>{try{const s=n();if(s===e)return;e=s,s===""?k():C({dataset:t})}catch{}},i=()=>{if(t.previewMount){C({dataset:t});return}o(),setInterval(o,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",i):i()}return v.mount=C,v.readOfficeService=w,v.readOfficeToken=y,v.unmount=k,Object.defineProperty(v,Symbol.toStringTag,{value:"Module"}),v})({});
