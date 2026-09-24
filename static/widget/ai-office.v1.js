var AIOffice=(function(v){"use strict";const N=`
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
`;function R(e){const t=document.createElement("div");t.setAttribute("data-ai-office-host",""),t.style.all="initial",e?(getComputedStyle(e).position==="static"&&(e.style.position="relative"),t.style.position="absolute",t.style.inset="0",e.appendChild(t)):document.body.appendChild(t);const n=t.attachShadow({mode:"closed"}),o=document.createElement("style");return o.textContent=N,n.appendChild(o),{host:t,root:n}}function $(e){const t=document.createElement("link");t.rel="stylesheet",t.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",e.appendChild(t)}function M(e){const t=c("button","launcher");t.type="button",t.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน");const n=c("div","panel");n.dataset.open="false";const o=c("div","head"),s=c("div","avatar"),i=c("div"),d=c("div","name");i.appendChild(d);const h=c("div","site"),f=c("button","x");f.type="button",f.textContent="×",f.setAttribute("aria-label","ปิด"),o.append(s,i,h,f);const u=c("div","log"),p=c("div","foot"),r=c("input");r.type="text",r.placeholder="พิมพ์คำถาม…";const l=c("button");return l.type="button",l.textContent="↑",p.append(r,l),n.append(o,u,p),e.append(t,n),f.addEventListener("click",()=>n.dataset.open="false"),{launcher:t,panel:n,head:{avatar:s,name:d,site:h},log:u,input:r,send:l}}function w(e,t,n){const o=c("div","row "+t),s=c("div","bubble");s.textContent=n,o.appendChild(s),e.appendChild(o),e.scrollTop=e.scrollHeight}function z(e,t){const n=c("div","note");n.textContent=t,e.appendChild(n),e.scrollTop=e.scrollHeight}function P(e,t){const{position:n,offset_x:o,offset_y:s}=t.placement,i=n==="bottom-left";for(const d of[e.launcher,e.panel])d.style.left="",d.style.right="";e.launcher.style.bottom=x(s),e.panel.style.bottom=x(s+68),i?(e.launcher.style.left=x(o),e.panel.style.left=x(o)):(e.launcher.style.right=x(o),e.panel.style.right=x(o))}function F(e,t){e.head.name.textContent=t.display_name||"ผู้ช่วยหลังบ้าน",e.head.site.textContent=t.service_label||t.service_id||"",e.head.avatar.textContent="",e.launcher.textContent="",t.avatar_url?(e.head.avatar.appendChild(A(t.avatar_url)),e.launcher.appendChild(A(t.avatar_url))):(e.head.avatar.textContent="AI",e.launcher.textContent="AI")}function A(e){const t=document.createElement("img");return t.src=e,t.alt="",t.referrerPolicy="no-referrer",t}function x(e){return`${e|0}px`}function c(e,t){const n=document.createElement(e);return t&&(n.className=t),n}const U=["avatar_url","display_name","greeting","theme","placement","service_label","is_hidden"],K=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],G="auth_token",H="web-service";function y(){try{const e=localStorage.getItem(G);if(!e)return"";const t=JSON.parse(e);if(!(t!=null&&t.value))return"";const n=Math.floor(Date.now()/1e3);return typeof t.expiration=="number"&&t.expiration<=n?"":t.value}catch{return""}}function E(){var e;try{return(e=localStorage.getItem(H))!=null?e:""}catch{return""}}const _={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}};let a=null;function k(){window.removeEventListener("message",D),a==null||a.host.remove(),a=null}async function C(e={}){var p,r,l,O,L;k();const t=(p=e.dataset)!=null?p:{};for(const g of K)t[g]&&(console.warn(`[ai-office] ไม่รับ data-${Y(g)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete t[g]);const n=(r=t.previewMount)!=null?r:"",o=n!=="",s=(O=(l=e.apiBase)!=null?l:t.apiBase)!=null?O:"";let i;if(o)i={..._,...(L=e.bootstrap)!=null?L:{}};else if(e.bootstrap)i={..._,...e.bootstrap};else{const g=await V(s,e.onFetch);if(!g)return;i={..._,...g}}if(!o&&!i.enabled)return;const d=o?document.querySelector(n):null;if(o&&!d){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:h,root:f}=R(d);o&&h.setAttribute("data-preview","true"),$(f);const u=M(f);a={ui:u,host:h,cfg:i,preview:o},I(i),u.launcher.addEventListener("click",()=>S()),u.send.addEventListener("click",()=>B(e)),u.input.addEventListener("keydown",g=>{g.key==="Enter"&&B(e)}),o&&(u.panel.dataset.open="true",window.addEventListener("message",D)),j()}function I(e){a&&(a.cfg=e,a.host.setAttribute("data-theme",W(e.theme)),F(a.ui,e),P(a.ui,e),a.ui.launcher.style.display=e.is_hidden?"none":"grid",a.ui.log.textContent="",e.greeting&&w(a.ui.log,"ai",e.greeting),z(a.ui.log,"ยังต่อกับข้อมูลจริงไม่ได้ — รอบนี้ทดสอบการติดตั้งและการตั้งค่าเท่านั้น"))}function D(e){if(e.origin!==window.location.origin)return;const t=e.data;if(!t||t.type!=="ai-office:preview-config"||!t.config||!a||!a.preview)return;const n={};for(const o of U)o in t.config&&(n[o]=t.config[o]);I({...a.cfg,...n})}async function V(e,t){var i,d,h,f,u;const n=y(),o=E();if(!n||!o)return m(!n&&!o?'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน':n?'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน':'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'),null;const s=`${e}/api/ai/widget/service/${encodeURIComponent(o)}/bootstrap`;t==null||t({url:s});try{const p=await fetch(s,{headers:{Authorization:`Bearer ${n}`}}),r=await p.json().catch(()=>null);if(!p.ok)return m(`เซิร์ฟเวอร์ตอบ ${p.status} ${(i=r==null?void 0:r.message)!=null?i:""} — ${(d=r==null?void 0:r.error)!=null?d:""}`.trim(),T[(h=r==null?void 0:r.message)!=null?h:""]),null;const l=(f=r==null?void 0:r.payload)!=null?f:null;return l&&!l.enabled&&m(`ยังไม่เปิดใช้งาน (reason: ${l.reason})`,T[(u=l.reason)!=null?u:""]),l}catch(p){return m(`เรียก ${s} ไม่สำเร็จ — ${p.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function m(e,t){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${e}`+(t?`
           → ${t}`:""))}const T={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "โดเมนที่อนุญาต" ของ office ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน",SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function S(e){if(!a)return;const t=e!=null?e:a.ui.panel.dataset.open!=="true";a.ui.panel.dataset.open=String(t),t&&a.ui.input.focus()}function B(e){var n;if(!a)return;const t=a.ui.input.value.trim();t&&(a.ui.input.value="",w(a.ui.log,"me",t),(n=e.onFetch)==null||n.call(e,{url:"(chat ยังไม่เปิดใช้ในรอบนี้)",body:{text:t}}),w(a.ui.log,"ai","ตอนนี้ผมยังตอบคำถามไม่ได้ครับ — รอบนี้ติดตั้งและตั้งค่าได้แล้ว ส่วนการตอบจากข้อมูลจริงจะมาในรอบถัดไป"))}function W(e){var t,n;return e==="light"||e==="dark"?e:(n=(t=window.matchMedia)==null?void 0:t.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function Y(e){return e.replace(/[A-Z]/g,t=>"-"+t.toLowerCase())}function j(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>S(!0),close:()=>S(!1),__hasLauncher:()=>!!a&&a.ui.launcher.style.display!=="none",__isOpen:()=>!!a&&a.ui.panel.dataset.open==="true",__launcherStyle:()=>a?a.ui.launcher.style:{},__config:()=>a?{...a.cfg}:{},__host:()=>{var t;return(t=a==null?void 0:a.host)!=null?t:null}}),configurable:!0})}const b=document.currentScript;if(b){const e={};for(const i in b.dataset)e[i]=b.dataset[i];b.src&&(e.apiBase||(e.apiBase=new URL(b.src,location.href).origin));let t=null;const n=()=>y()?E()||"\0":"",o=()=>{try{const i=n();if(i===t)return;t=i,i===""?k():C({dataset:e})}catch{}},s=()=>{if(e.previewMount){C({dataset:e});return}o(),setInterval(o,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",s):s()}return v.mount=C,v.readOfficeService=E,v.readOfficeToken=y,v.unmount=k,Object.defineProperty(v,Symbol.toStringTag,{value:"Module"}),v})({});
