var AIOffice=(function(b){"use strict";const M=`
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
`;function z(e){const t=document.createElement("div");t.setAttribute("data-ai-office-host",""),t.style.all="initial",e?(getComputedStyle(e).position==="static"&&(e.style.position="relative"),t.style.position="absolute",t.style.inset="0",e.appendChild(t)):document.body.appendChild(t);const n=t.attachShadow({mode:"closed"}),a=document.createElement("style");return a.textContent=M,n.appendChild(a),{host:t,root:n}}function F(e){const t=document.createElement("link");t.rel="stylesheet",t.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",e.appendChild(t)}function P(e){const t=s("button","launcher");t.type="button",t.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน");const n=s("div","panel");n.dataset.open="false";const a=s("div","head"),r=s("div","avatar"),c=s("div"),l=s("div","name");c.appendChild(l);const p=s("div","site"),f=s("button","x");f.type="button",f.textContent="×",f.setAttribute("aria-label","ปิด"),a.append(r,c,p,f);const h=s("div","log"),u=s("div","foot"),d=s("input");d.type="text",d.placeholder="พิมพ์คำถาม…";const i=s("button");return i.type="button",i.textContent="↑",u.append(d,i),n.append(a,h,u),e.append(t,n),f.addEventListener("click",()=>n.dataset.open="false"),{launcher:t,panel:n,head:{avatar:r,name:l,site:p},log:h,input:d,send:i}}function w(e,t,n){const a=s("div","row "+t),r=s("div","bubble");r.textContent=n,a.appendChild(r),e.appendChild(a),e.scrollTop=e.scrollHeight}function U(e,t){const n=s("div","note");n.textContent=t,e.appendChild(n),e.scrollTop=e.scrollHeight}function K(e,t){const{position:n,offset_x:a,offset_y:r}=t.placement,c=n==="bottom-left";for(const l of[e.launcher,e.panel])l.style.left="",l.style.right="";e.launcher.style.bottom=x(r),e.panel.style.bottom=x(r+68),c?(e.launcher.style.left=x(a),e.panel.style.left=x(a)):(e.launcher.style.right=x(a),e.panel.style.right=x(a))}function W(e,t){e.head.name.textContent=t.display_name||"ผู้ช่วยหลังบ้าน",e.head.site.textContent=t.service_label||t.service_id||"",e.head.avatar.textContent="",e.launcher.textContent="",t.avatar_url?(e.head.avatar.appendChild(I(t.avatar_url)),e.launcher.appendChild(I(t.avatar_url))):(e.head.avatar.textContent="AI",e.launcher.textContent="AI")}function I(e){const t=document.createElement("img");return t.src=e,t.alt="",t.referrerPolicy="no-referrer",t}function x(e){return`${e|0}px`}function s(e,t){const n=document.createElement(e);return t&&(n.className=t),n}const H=["avatar_url","display_name","greeting","theme","placement","service_label","is_hidden"],V=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],j="auth_token",Y="web-service";function _(){try{const e=localStorage.getItem(j);if(!e)return"";const t=JSON.parse(e);if(!(t!=null&&t.value))return"";const n=Math.floor(Date.now()/1e3);return typeof t.expiration=="number"&&t.expiration<=n?"":t.value}catch{return""}}function k(){var e;try{return(e=localStorage.getItem(Y))!=null?e:""}catch{return""}}const E={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}};let o=null;function C(){window.removeEventListener("message",B),o==null||o.host.remove(),o=null}async function S(e={}){var d,i,g,N,R,$;C();const t=(d=e.dataset)!=null?d:{};for(const v of V)t[v]&&(console.warn(`[ai-office] ไม่รับ data-${J(v)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete t[v]);const n=(i=t.previewMount)!=null?i:"",a=n!=="",r=(N=(g=e.apiBase)!=null?g:t.apiBase)!=null?N:"",c=(R=t.publicKey)!=null?R:"";let l;if(a)l={...E,...($=e.bootstrap)!=null?$:{}};else if(e.bootstrap)l={...E,...e.bootstrap};else{const v=await G(r,c,e.onFetch);if(!v)return;l={...E,...v}}if(!a&&!l.enabled)return;const p=a?document.querySelector(n):null;if(a&&!p){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:f,root:h}=z(p);a&&f.setAttribute("data-preview","true"),F(h);const u=P(h);o={ui:u,host:f,cfg:l,preview:a},O(l),u.launcher.addEventListener("click",()=>A()),u.send.addEventListener("click",()=>L(e)),u.input.addEventListener("keydown",v=>{v.key==="Enter"&&L(e)}),a&&(u.panel.dataset.open="true",window.addEventListener("message",B)),X()}function O(e){o&&(o.cfg=e,o.host.setAttribute("data-theme",q(e.theme)),W(o.ui,e),K(o.ui,e),o.ui.launcher.style.display=e.is_hidden?"none":"grid",o.ui.log.textContent="",e.greeting&&w(o.ui.log,"ai",e.greeting),U(o.ui.log,"ยังต่อกับข้อมูลจริงไม่ได้ — รอบนี้ทดสอบการติดตั้งและการตั้งค่าเท่านั้น"))}function B(e){if(e.origin!==window.location.origin)return;const t=e.data;if(!t||t.type!=="ai-office:preview-config"||!t.config||!o||!o.preview)return;const n={};for(const a of H)a in t.config&&(n[a]=t.config[a]);O({...o.cfg,...n})}async function G(e,t,n){var l,p,f,h,u;if(!t)return y("ไม่พบ public key ใน URL ของ script","snippet ต้องเป็น .../widget/v1/<public_key>/ai-office.js"),null;const a=_(),r=k();if(!a||!r)return y(!a&&!r?'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน':a?'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน':'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'),null;const c=`${e}/api/ai/office/${encodeURIComponent(t)}/service/${encodeURIComponent(r)}/bootstrap`;n==null||n({url:c});try{const d=await fetch(c,{headers:{Authorization:`Bearer ${a}`}}),i=await d.json().catch(()=>null);if(!d.ok)return y(`เซิร์ฟเวอร์ตอบ ${d.status} ${(l=i==null?void 0:i.message)!=null?l:""} — ${(p=i==null?void 0:i.error)!=null?p:""}`.trim(),D[(f=i==null?void 0:i.message)!=null?f:""]),null;const g=(h=i==null?void 0:i.payload)!=null?h:null;return g&&!g.enabled&&y(`ยังไม่เปิดใช้งาน (reason: ${g.reason})`,D[(u=g.reason)!=null?u:""]),g}catch(d){return y(`เรียก ${c} ไม่สำเร็จ — ${d.message}`,"backend ทำงานอยู่ไหม และโดเมนนี้อยู่ใน allowed_origins หรือยัง"),null}}function y(e,t){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${e}`+(t?`
           → ${t}`:""))}const D={ORIGIN_NOT_ALLOWED:'เพิ่มโดเมนของหน้านี้ลงใน "โดเมนที่อนุญาต" ของ office ที่คอนโซล',SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน",NOT_FOUND:"public key ใน snippet ไม่ตรงกับ office ไหนเลย — ถูกลบหรือ rotate key ไปแล้วหรือเปล่า",SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ office-api ไม่ได้ — ตรวจ backoffice_api_url ของ office",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับ key ใน snippet",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function A(e){if(!o)return;const t=e!=null?e:o.ui.panel.dataset.open!=="true";o.ui.panel.dataset.open=String(t),t&&o.ui.input.focus()}function L(e){var n;if(!o)return;const t=o.ui.input.value.trim();t&&(o.ui.input.value="",w(o.ui.log,"me",t),(n=e.onFetch)==null||n.call(e,{url:"(chat ยังไม่เปิดใช้ในรอบนี้)",body:{text:t}}),w(o.ui.log,"ai","ตอนนี้ผมยังตอบคำถามไม่ได้ครับ — รอบนี้ติดตั้งและตั้งค่าได้แล้ว ส่วนการตอบจากข้อมูลจริงจะมาในรอบถัดไป"))}function q(e){var t,n;return e==="light"||e==="dark"?e:(n=(t=window.matchMedia)==null?void 0:t.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function J(e){return e.replace(/[A-Z]/g,t=>"-"+t.toLowerCase())}function X(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>A(!0),close:()=>A(!1),__hasLauncher:()=>!!o&&o.ui.launcher.style.display!=="none",__isOpen:()=>!!o&&o.ui.panel.dataset.open==="true",__launcherStyle:()=>o?o.ui.launcher.style:{},__config:()=>o?{...o.cfg}:{},__host:()=>{var t;return(t=o==null?void 0:o.host)!=null?t:null}}),configurable:!0})}function T(e){const t=new URL(e,location.href).pathname.split("/").filter(Boolean),n=t.lastIndexOf("ai-office.js");return n>0?decodeURIComponent(t[n-1]):""}const m=document.currentScript;if(m){const e={};for(const c in m.dataset)e[c]=m.dataset[c];m.src&&(e.apiBase||(e.apiBase=new URL(m.src,location.href).origin),e.publicKey||(e.publicKey=T(m.src)));let t=null;const n=()=>_()?k()||"\0":"",a=()=>{try{const c=n();if(c===t)return;t=c,c===""?C():S({dataset:e})}catch{}},r=()=>{if(e.previewMount){S({dataset:e});return}a(),setInterval(a,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",r):r()}return b.keyFromScriptURL=T,b.mount=S,b.readOfficeService=k,b.readOfficeToken=_,b.unmount=C,Object.defineProperty(b,Symbol.toStringTag,{value:"Module"}),b})({});
