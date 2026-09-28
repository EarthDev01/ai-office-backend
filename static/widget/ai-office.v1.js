var AIOffice=(function(_){"use strict";var re;const ce=`
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
  width:58px; height:58px; border-radius:50%; padding:0;
  border:0; cursor:pointer;
  color:var(--on-accent,#fff);
  /* แสงเงาบนสีหลัก — ให้ปุ่มดูนูน ไม่แบน (สีหลักตั้งจากคอนโซลได้) */
  background:
    radial-gradient(120% 90% at 30% 18%, rgba(255,255,255,.34), rgba(255,255,255,0) 58%),
    linear-gradient(160deg, rgba(255,255,255,0) 45%, rgba(0,0,0,.18)),
    var(--accent);
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
.row.me .bubble{background:var(--accent);color:var(--on-accent,#fff);border-color:transparent}
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
  background:var(--accent);color:var(--on-accent,#fff);font-family:var(--font-head);font-weight:600;
}
.foot .send:disabled,.foot textarea:disabled{opacity:.5;cursor:default}

/* ---- โหมด preview: อยู่ในกล่อง ไม่ลอยมุมจอ ---- */
:host([data-preview="true"]) .launcher,
:host([data-preview="true"]) .panel{position:absolute}
:host([data-preview="true"]) .panel{
  inset:auto; top:12px; left:12px; right:12px; bottom:12px;
  width:auto; height:auto; display:flex;
}
`;function le(t){const e=document.createElement("div");e.setAttribute("data-ai-office-host",""),e.style.all="initial",t?(getComputedStyle(t).position==="static"&&(t.style.position="relative"),e.style.position="absolute",e.style.inset="0",t.appendChild(e)):document.body.appendChild(e);const n=e.attachShadow({mode:"closed"}),a=document.createElement("style");return a.textContent=ce,n.appendChild(a),{host:e,root:n}}function de(t){const e=document.createElement("link");e.rel="stylesheet",e.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",t.appendChild(e)}const ue=2e3;function fe(t){const e=u("button","launcher");e.type="button",e.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน"),e.setAttribute("aria-expanded","false"),e.dataset.open="false";const n=u("div","panel");n.dataset.open="false",n.setAttribute("role","dialog"),n.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const a=u("div","head"),o=u("div","avatar"),i=u("div","namewrap"),r=u("div","name");i.appendChild(r);const l=u("div","site");l.title="เว็บที่กำลังคุยอยู่";const c=u("button","x");c.type="button",c.textContent="×",c.setAttribute("aria-label","ปิด"),a.append(o,i,l,c);const p=u("div","log");p.setAttribute("aria-live","polite");const s=u("div","foot"),f=u("textarea");f.rows=1,f.maxLength=ue,f.placeholder="พิมพ์คำถาม…",f.setAttribute("aria-label","คำถาม");const h=u("button","send");h.type="button",h.textContent="↑",h.setAttribute("aria-label","ส่ง"),s.append(f,h),n.append(a,p,s),t.append(e,n);const g={launcher:e,panel:n,head:{avatar:o,name:r,site:l},log:p,input:f,send:h};return c.addEventListener("click",()=>R(g,!1)),g}function R(t,e){t.panel.dataset.open=String(e),t.launcher.dataset.open=String(e),t.launcher.setAttribute("aria-expanded",String(e)),t.launcher.setAttribute("aria-label",e?"ปิดผู้ช่วยหลังบ้าน":"เปิดผู้ช่วยหลังบ้าน")}const H="http://www.w3.org/2000/svg";function M(t,e){const n=document.createElementNS(H,"svg");n.setAttribute("viewBox","0 0 24 24"),n.setAttribute("aria-hidden","true"),n.setAttribute("class",t);for(const a of e){const o=document.createElementNS(H,"path");o.setAttribute("d",a.d),a.fill&&o.setAttribute("class","fill"),n.appendChild(o)}return n}const G=[{d:"M3.5 6.25A3.25 3.25 0 0 1 6.75 3h10.5a3.25 3.25 0 0 1 3.25 3.25v7.5A3.25 3.25 0 0 1 17.25 17H11.5l-4.1 3.4c-.7.58-1.9.1-1.9-.83V17A3.25 3.25 0 0 1 3.5 13.75z"},{d:"M12 5.6l1.05 2.6 2.6 1.05-2.6 1.05L12 12.9l-1.05-2.6-2.6-1.05 2.6-1.05z",fill:!0}],pe=[{d:"M7 7l10 10M17 7L7 17"}];function he(t,e){t.launcher.textContent="";const n=u("span","face");e?n.appendChild(K(e)):n.appendChild(M("ico",G)),t.launcher.append(n,M("ico close",pe))}function x(t,e,n,a=""){const o=u("div","row "+e),i=u("div","bubble"+(a?" "+a:"")),r=u("div","txt");return r.textContent=n,i.appendChild(r),o.appendChild(i),t.appendChild(o),k(t),i}function ge(t,e){const n=u("div","sys");return n.textContent=e,t.appendChild(n),k(t),n}function J(t,e){const n=u("div","row ai"),a=u("div","bubble load"),o=u("span");o.textContent=e;const i=u("span","dots");return i.append(u("span"),u("span"),u("span")),a.append(o,i),n.appendChild(a),t.appendChild(n),k(t),{set:r=>o.textContent=r,remove:()=>n.remove()}}function q(t){var i,r,l,c,p;const e=u("div","datacard k-"+ke(t.kind));if(t.title){const s=u("div","dc-title");s.textContent=t.title,e.appendChild(s)}for(const s of(i=t.fields)!=null?i:[]){const f=u("div","dc-row"),h=u("span");h.textContent=s.label;const g=u("b");g.textContent=s.display,f.append(h,g),e.appendChild(f)}if(t.table&&((r=t.table.rows)!=null&&r.length)){const s=u("div","dc-tablewrap"),f=u("table","dc-table"),h=u("thead"),g=u("tr");for(const y of(l=t.table.columns)!=null?l:[]){const m=u("th");m.textContent=y.label,g.appendChild(m)}h.appendChild(g);const b=u("tbody");for(const y of t.table.rows){const m=u("tr");for(const v of y){const O=u("td");O.textContent=(c=v==null?void 0:v.display)!=null?c:"",m.appendChild(O)}b.appendChild(m)}f.append(h,b),s.appendChild(f),e.appendChild(s)}if(t.note){const s=u("div","dc-note");s.textContent=t.note,e.appendChild(s)}const n=u("div","src"),a=u("span");a.textContent=t.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+be(t.fetched_at)+(t.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),n.appendChild(a);const o=me((p=t.link)==null?void 0:p.path);if(t.link&&o){const s=u("a");s.href=o,s.textContent=(t.link.label||"เปิดหน้าจริง")+" →",s.target="_self",s.rel="noopener",n.appendChild(s)}return e.appendChild(n),e}function be(t){const e=new Date(t);if(isNaN(e.getTime()))return"-";const n={timeZone:"Asia/Bangkok"};try{const a=e.toLocaleTimeString("th-TH",{...n,hour:"2-digit",minute:"2-digit",hour12:!1}),o=e.toLocaleDateString("en-CA",n),i=new Date().toLocaleDateString("en-CA",n);if(o===i)return a;const[,r,l]=o.split("-");return`${l}/${r} ${a}`}catch{return e.toISOString().slice(0,16).replace("T"," ")}}function me(t){if(!t)return"";const e=t.trim();return e.startsWith("#")?e:!e.startsWith("/")||e.startsWith("//")||e.includes("\\")?"":e}function xe(t,e){const{position:n,offset_x:a,offset_y:o}=e.placement,i=n==="bottom-left";for(const r of[t.launcher,t.panel])r.style.left="",r.style.right="";t.launcher.style.bottom=S(o),t.panel.style.bottom=S(o+68),i?(t.launcher.style.left=S(a),t.panel.style.left=S(a)):(t.launcher.style.right=S(a),t.panel.style.right=S(a))}function ye(t,e){t.head.name.textContent=e.display_name||"ผู้ช่วยหลังบ้าน",t.head.site.textContent=e.service_label||e.service_id||"",t.head.avatar.textContent="",e.avatar_url?t.head.avatar.appendChild(K(e.avatar_url)):t.head.avatar.appendChild(M("ico",G)),he(t,e.avatar_url)}const ve=/^#[0-9a-f]{6}$/i;function we(t,e,n){const a=["--accent","--accent-soft","--on-accent"];if(!e||!ve.test(e)){for(const c of a)t.style.removeProperty(c);return}const o=[1,3,5].map(c=>parseInt(e.slice(c,c+2),16)),i=n==="dark"?[21,33,35]:[255,255,255],r=c=>"#"+o.map((p,s)=>Math.round(p*c+i[s]*(1-c)).toString(16).padStart(2,"0")).join(""),l=o.map(c=>c/255).map(c=>c<=.03928?c/12.92:((c+.055)/1.055)**2.4).reduce((c,p,s)=>c+p*[.2126,.7152,.0722][s],0);t.style.setProperty("--accent",e),t.style.setProperty("--accent-soft",r(n==="dark"?.25:.16)),t.style.setProperty("--on-accent",l>.45?"#13282B":"#ffffff")}function k(t){t.scrollTop=t.scrollHeight}function K(t){const e=document.createElement("img");return e.src=t,e.alt="",e.referrerPolicy="no-referrer",e}function ke(t){return String(t||"").replace(/[^a-z_]/g,"")}function S(t){return`${t|0}px`}function u(t,e){const n=document.createElement(t);return e&&(n.className=e),n}const Ce=["avatar_url","display_name","greeting","theme","accent_color","placement","service_label","is_hidden"],_e=2*1024*1024;function Se(t,e,n){if(!t||typeof e!="string"||!e.startsWith("/")||e.startsWith("//")||e.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(e)||e.includes("://"))return null;let a;try{a=new URL(t)}catch{return null}const o=a.pathname.replace(/\/+$/,"");let i;try{i=new URL(a.origin+o+e)}catch{return null}if(i.origin!==a.origin||!(i.pathname===o||i.pathname.startsWith(o+"/")))return null;if(n)for(const[r,l]of Object.entries(n))i.searchParams.set(r,String(l));return i.toString()}async function V(t,e,n,a="Bearer"){var c;if(!n)return{status:401,body:""};const o=String(e.method||"").toUpperCase();if(o!=="GET"&&o!=="POST")return{status:0,body:""};const i=Se(t,e.path,e.query);if(!i)return{status:0,body:""};const r=new AbortController,l=setTimeout(()=>r.abort(),Math.max(1e3,(c=e.timeout_ms)!=null?c:2e4));try{const p={method:o,credentials:"omit",signal:r.signal,headers:{Authorization:`${a} ${n}`,Accept:"application/json"}};o==="POST"&&e.body!==void 0&&e.body!==null&&(p.headers["Content-Type"]="application/json",p.body=JSON.stringify(e.body));const s=await fetch(i,p),f=await s.text();return f.length>_e?{status:0,body:""}:{status:s.status,body:f}}catch{return{status:0,body:""}}finally{clearTimeout(l)}}const I=new Set(["","null","undefined"]);function X(t){try{if(t==="localStorage")return localStorage;if(t==="sessionStorage")return sessionStorage}catch{}return null}function Y(t){var e,n;if(!t)return"";try{const a=(n=(e=X(t.source))==null?void 0:e.getItem(t.key))!=null?n:"";if(I.has(a.trim()))return"";if(t.format!=="json-expiration")return a.trim();const o=JSON.parse(a),i=o==null?void 0:o[t.value_field||"value"];if(typeof i!="string"||I.has(i))return"";const r=o[t.expiration_field||"expiration"];return typeof r=="number"&&r<=Math.floor(Date.now()/1e3)?"":i}catch{return""}}function U(t){var e,n,a,o;if(!t)return"";try{let i="";if(t.source==="query"){if(i=(e=new URLSearchParams(location.search).get(t.key))!=null?e:"",!i){const r=location.hash.indexOf("?");r>=0&&(i=(n=new URLSearchParams(location.hash.slice(r+1)).get(t.key))!=null?n:"")}}else i=(o=(a=X(t.source))==null?void 0:a.getItem(t.key))!=null?o:"";if(i=i.trim(),I.has(i))return"";if(t.encoding==="base64"){const r=atob(i).trim();return I.has(r)?"":r}return i}catch{return""}}function Q(t){return t?t.source==="query"?`?${t.key}=`:`${t.source}["${t.key}"]`:"(ไม่ทราบ)"}function Ae(t){const e=t.split(".")[1];if(!e)return null;try{const n=e.replace(/-/g,"+").replace(/_/g,"/"),a=atob(n+"=".repeat((4-n.length%4)%4)),o=Uint8Array.from(a,i=>i.charCodeAt(0));return JSON.parse(new TextDecoder().decode(o))}catch{return null}}function Z(t,e){const n=$(t,e.path);if(!Array.isArray(n))return[];const a=[];for(const o of n){if(e.where_field&&$(o,e.where_field)!==e.where_value)continue;const i=e.pluck?$(o,e.pluck):o;typeof i=="string"&&i&&a.push(i)}return a}function $(t,e){let n=t;if(!e)return n;for(const a of e.split(".")){if(!n||typeof n!="object")return;n=n[a]}return n}const Ee=180*1e3;class Te{constructor(e,n=""){this.apiBase=e,this.hostAPIBase=n,this.cfg=null,this.loading=null,this.cur=null}hostApiBase(){var e;return this.hostAPIBase||((e=this.cfg)==null?void 0:e.host_api_base)||""}readToken(){var e;return Y((e=this.cfg)==null?void 0:e.token)}authScheme(){var e;return((e=this.cfg)==null?void 0:e.auth_scheme)||"Bearer"}async pageConfig(){var e;return this.cfg?this.cfg:((e=this.loading)!=null||(this.loading=(async()=>{var o;const n=await fetch(`${this.apiBase}/api/ai/widget/page-config`),a=await n.json().catch(()=>null);if(!n.ok||!(a!=null&&a.payload))throw new ee((o=a==null?void 0:a.message)!=null?o:String(n.status));return this.cfg=a.payload,this.cfg})().finally(()=>{this.loading=null})),this.loading)}async ticket(e){var r;if(this.cur&&this.cur.service===e&&this.cur.expiresAt-Date.now()>Ee)return this.cur.ticket;const n=await this.pageConfig(),a=await this.readPermissions(n,e),o=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(e)}/browser-session`,{method:"POST",headers:{Authorization:`Bearer ${this.readToken()}`,"Content-Type":"application/json"},body:JSON.stringify({permissions:a})}),i=await o.json().catch(()=>null);if(!o.ok||!((r=i==null?void 0:i.payload)!=null&&r.ticket))throw new Error((i==null?void 0:i.error)||`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${o.status})`);return this.cur={service:e,ticket:i.payload.ticket,expiresAt:Date.now()+i.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(e,n){const a=e.identity,o=a==null?void 0:a.permissions;if(!(o!=null&&o.path))return[];if(a!=null&&a.permissions_token){const l=Y(a.permissions_token);return l?Z($(Ae(l),a.root),o):[]}const i=a==null?void 0:a.permissions_request;if(!i)return[];const r=await V(this.hostApiBase(),{id:"perm",method:"GET",path:i.split("{service}").join(encodeURIComponent(n))},this.readToken(),this.authScheme());if(r.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${r.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{return Z(JSON.parse(r.body),o)}catch{return[]}}}class ee extends Error{}class A extends Error{}async function Be(t){const{apiBase:e,service:n,session:a,on:o}=t,i=await a.ticket(n),r=`${e}/api/ai/widget/service/${encodeURIComponent(n)}`;let l=t.conversationID;const c=await fetch(`${r}/chat`,{method:"POST",headers:{Authorization:`Bearer ${i}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:l,text:t.text})});if(!c.ok||!c.body){c.status===401&&a.invalidate();const h=await c.json().catch(()=>null);throw new A((h==null?void 0:h.error)||`เซิร์ฟเวอร์ตอบ ${c.status}`)}const p=[],s=h=>p.push(V(a.hostApiBase(),h,a.readToken(),a.authScheme()).then(g=>fetch(`${r}/chat/relay/${encodeURIComponent(h.id)}`,{method:"POST",headers:{Authorization:`Bearer ${i}`,"Content-Type":"application/json"},body:JSON.stringify(g)})).then(()=>{}).catch(()=>{}));let f=null;if(await te(c.body,(h,g)=>{var y,m,v;const b=g;switch(h){case"status":typeof b.conversation_id=="string"&&(l=b.conversation_id),o.status(String((y=b.text)!=null?y:""));break;case"fetch":s(b);break;case"card":o.card(b);break;case"token":o.token(String((m=b.text)!=null?m:""));break;case"error":f=new A(String((v=b.message)!=null?v:"ผู้ช่วยตอบไม่สำเร็จ"));break}}),await Promise.all(p),f)throw f;return l}async function Oe(t){const e=await fetch(`${t.apiBase}/api/ai/admin/assistant`,{method:"POST",headers:{Authorization:`Bearer ${t.token}`,"Content-Type":"application/json"},body:JSON.stringify({messages:t.messages,page:t.page})});if(!e.ok||!e.body){const a=await e.json().catch(()=>null);throw new A((a==null?void 0:a.error)||`เซิร์ฟเวอร์ตอบ ${e.status}`)}let n=null;if(await te(e.body,(a,o)=>{var r,l,c;const i=o;a==="status"?t.on.status(String((r=i.text)!=null?r:"")):a==="card"?t.on.card(i):a==="token"?t.on.token(String((l=i.text)!=null?l:"")):a==="error"&&(n=new A(String((c=i.message)!=null?c:"ผู้ช่วยตอบไม่สำเร็จ")))}),n)throw n}async function te(t,e){const n=t.getReader(),a=new TextDecoder;let o="";for(;;){const{value:i,done:r}=await n.read();if(r)break;o+=a.decode(i,{stream:!0});let l;for(;(l=o.indexOf(`

`))>=0;){const c=o.slice(0,l);o=o.slice(l+2);let p="message",s="";for(const f of c.split(`
`))f.startsWith("event:")?p=f.slice(6).trim():f.startsWith("data:")&&(s+=f.slice(5).trim());s&&e(p,JSON.parse(s))}}}const Ie=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],F=new Map;function $e(){F.clear()}function ne(t,e=""){const n=t+"|"+e;let a=F.get(n);return a||(a=new Te(t,e),F.set(n,a)),a}const D={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}},De=20;let d=null;function P(){window.removeEventListener("message",oe),d==null||d.host.remove(),d=null}async function L(t={}){var f,h,g,b,y,m,v,O,se;P();const e=(f=t.dataset)!=null?f:{};for(const w of Ie)e[w]&&(console.warn(`[ai-office] ไม่รับ data-${ze(w)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete e[w]);const n=(h=e.previewMount)!=null?h:"",a=n!=="",o=(b=(g=t.apiBase)!=null?g:e.apiBase)!=null?b:"",i=ne(o,e.hostApiBase);let r;if(t.console)r={...D,display_name:(y=t.console.displayName)!=null?y:"ผู้ช่วย AI Office",greeting:(m=t.console.greeting)!=null?m:"สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)",service_label:(v=t.console.label)!=null?v:"คอนโซล",placement:{position:"bottom-right",offset_x:20,offset_y:20}};else if(a)r={...D,...(O=t.bootstrap)!=null?O:{}};else if(t.bootstrap)r={...D,...t.bootstrap};else{const w=await Pe(o,i,t.onFetch);if(!w)return;r={...D,...w}}if(!a&&!r.enabled)return;const l=a?document.querySelector(n):null;if(a&&!l){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:c,root:p}=le(l);a&&c.setAttribute("data-preview","true"),de(p);const s=fe(p);d={ui:s,host:c,cfg:r,preview:a,apiBase:o,session:i,conversationID:"",busy:!1,console:(se=t.console)!=null?se:null,history:[]},ae(r),s.launcher.addEventListener("click",()=>T()),s.send.addEventListener("click",()=>N(t)),s.input.addEventListener("keydown",w=>{const z=w;z.key==="Enter"&&!z.shiftKey&&!z.isComposing&&(z.preventDefault(),N(t))}),a&&(R(s,!0),window.addEventListener("message",oe)),t.console||Re()}function ae(t){if(!d)return;d.cfg=t;const e=Ne(t.theme);d.host.setAttribute("data-theme",e),we(d.host,t.accent_color,e),ye(d.ui,t),xe(d.ui,t),d.ui.launcher.style.display=t.is_hidden?"none":"grid",d.ui.log.textContent="",t.greeting&&x(d.ui.log,"ai",t.greeting),d.preview&&ge(d.ui.log,C.preview)}function oe(t){if(t.origin!==window.location.origin)return;const e=t.data;if(!e||e.type!=="ai-office:preview-config"||!e.config||!d||!d.preview)return;const n={};for(const a of Ce)a in e.config&&(n[a]=e.config[a]);ae({...d.cfg,...n})}async function Pe(t,e,n){var l,c,p,s,f;const a=await W(e);if(!a)return null;const o=e.readToken(),i=U(a.service);if(!o||!i){const h=Q(a.token),g=Q(a.service);return E(!o&&!i?`ไม่พบ ${h} และ ${g} — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน`:o?`ไม่พบ ${g} — ยังไม่ได้เลือกเว็บในหลังบ้าน (หน้านี้ไม่ได้ผูกกับเว็บใด)`:`ไม่พบ ${h} ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว`),null}const r=`${t}/api/ai/widget/service/${encodeURIComponent(i)}/bootstrap`;n==null||n({url:r});try{const h=await fetch(r,{headers:{Authorization:`Bearer ${o}`}}),g=await h.json().catch(()=>null);if(!h.ok)return E(`เซิร์ฟเวอร์ตอบ ${h.status} ${(l=g==null?void 0:g.message)!=null?l:""} — ${(c=g==null?void 0:g.error)!=null?c:""}`.trim(),j[(p=g==null?void 0:g.message)!=null?p:""]),null;const b=(s=g==null?void 0:g.payload)!=null?s:null;return b&&!b.enabled&&E(`ยังไม่เปิดใช้งาน (reason: ${b.reason})`,j[(f=b.reason)!=null?f:""]),b}catch(h){return E(`เรียก ${r} ไม่สำเร็จ — ${h.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}async function W(t){var e;try{return await t.pageConfig()}catch(n){const a=n instanceof ee?n.message:"";return E(`โหลดการตั้งค่าของหน้านี้ไม่สำเร็จ (${a||n.message})`,(e=j[a])!=null?e:"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function E(t,e){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${t}`+(e?`
           → ${e}`:""))}const j={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์เปิดเว็บนี้ในหลังบ้าน (รายชื่อเว็บของบัญชี)",KIND_NOT_SUPPORTED:'ชนิดหลังบ้านของ domain นี้ยังไม่มี connector — ตรวจ "ชนิดหลังบ้าน" ที่คอนโซล',SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function T(t){if(!d)return;const e=t!=null?t:d.ui.panel.dataset.open!=="true";R(d.ui,e),e&&d.ui.input.focus()}const C={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)"};async function N(t){var p;const e=d;if(!e||e.busy||e.ui.input.disabled)return;const n=e.ui.input.value.trim();if(!n)return;if(e.ui.input.value="",x(e.ui.log,"me",n),e.preview){x(e.ui.log,"ai",C.preview);return}if(e.console){await Le(e,e.console,n);return}const a=await W(e.session),o=a?U(a.service):"";if(!o){x(e.ui.log,"ai",C.unavailable,"err");return}(p=t.onFetch)==null||p.call(t,{url:`${e.apiBase}/api/ai/widget/service/${encodeURIComponent(o)}/chat`,body:{text:n}}),e.busy=!0,e.ui.send.disabled=!0;const i=J(e.ui.log,"กำลังส่งคำถาม…");let r=null,l=null;const c=()=>{if(r)return;const s=x(e.ui.log,"ai","");r=s.querySelector(".txt"),l=u("div","cards"),s.appendChild(l)};try{e.conversationID=await Be({apiBase:e.apiBase,service:o,session:e.session,conversationID:e.conversationID,text:n,on:{status:s=>i.set(s||"กำลังทำงาน…"),card:s=>{c(),l.appendChild(q(s)),k(e.ui.log)},token:s=>{var f;i.remove(),c(),r.textContent=((f=r.textContent)!=null?f:"")+s,k(e.ui.log)}}}),r||x(e.ui.log,"ai",C.empty)}catch(s){const f=s instanceof A?s.message:C.unavailable;x(e.ui.log,"ai",f,"err")}finally{i.remove(),e.busy=!1,e.ui.send.disabled=!1}}async function Le(t,e,n){var c,p;t.history.push({role:"user",text:n}),t.busy=!0,t.ui.send.disabled=!0;const a=J(t.ui.log,"กำลังส่งคำถาม…");let o=null,i=null,r="";const l=()=>{if(o)return;const s=x(t.ui.log,"ai","");o=s.querySelector(".txt"),i=u("div","cards"),s.appendChild(i)};try{await Oe({apiBase:t.apiBase,token:e.getToken(),messages:t.history.slice(-De),page:(p=(c=e.getPage)==null?void 0:c.call(e))!=null?p:"",on:{status:s=>a.set(s||"กำลังทำงาน…"),card:s=>{l(),i.appendChild(q(s)),k(t.ui.log)},token:s=>{a.remove(),l(),r+=s,o.textContent=r,k(t.ui.log)}}}),r?t.history.push({role:"assistant",text:r}):x(t.ui.log,"ai",C.empty)}catch(s){t.history.pop();const f=s instanceof A?s.message:C.unavailable;x(t.ui.log,"ai",f,"err")}finally{a.remove(),t.busy=!1,t.ui.send.disabled=!1}}function Ne(t){var e,n;return t==="light"||t==="dark"?t:(n=(e=window.matchMedia)==null?void 0:e.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function ze(t){return t.replace(/[A-Z]/g,e=>"-"+e.toLowerCase())}function ie(t){Object.defineProperty(window,"__aiOfficeConsole",{value:Object.freeze({mount:n=>L({apiBase:t,console:n}),unmount:P,open:()=>T(!0),close:()=>T(!1),__logText:()=>{var n;return(n=d==null?void 0:d.ui.log.textContent)!=null?n:""},__send:async n=>{d&&(d.ui.input.value=n,await N({}))}}),configurable:!0})}function Re(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>T(!0),close:()=>T(!1),__hasLauncher:()=>!!d&&d.ui.launcher.style.display!=="none",__isOpen:()=>!!d&&d.ui.panel.dataset.open==="true",__launcherStyle:()=>d?d.ui.launcher.style:{},__config:()=>d?{...d.cfg}:{},__host:()=>{var e;return(e=d==null?void 0:d.host)!=null?e:null},__send:async e=>{d&&(d.ui.input.value=e,await N({}))},__logText:()=>{var e;return(e=d==null?void 0:d.ui.log.textContent)!=null?e:""},__conversationID:()=>{var e;return(e=d==null?void 0:d.conversationID)!=null?e:""}}),configurable:!0})}const B=document.currentScript;if(B){const t={};for(const l in B.dataset)t[l]=B.dataset[l];B.src&&(t.apiBase||(t.apiBase=new URL(B.src,location.href).origin));const e=ne((re=t.apiBase)!=null?re:"",t.hostApiBase);let n=null,a=0,o=null;const i=()=>{try{if(!n){if(Date.now()<a)return;a=Date.now()+3e4,W(e).then(c=>{n=c,c&&i()});return}const l=e.readToken()?U(n.service)||"\0":"";if(l===o)return;o=l,l===""?P():L({dataset:t})}catch{}},r=()=>{var l;if(t.consoleMode!==void 0){ie((l=t.apiBase)!=null?l:"");return}if(t.previewMount){L({dataset:t});return}i(),setInterval(i,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",r):r()}return _.__resetSessions=$e,_.installConsoleGlobal=ie,_.mount=L,_.unmount=P,Object.defineProperty(_,Symbol.toStringTag,{value:"Module"}),_})({});
