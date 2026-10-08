var AIOffice=(function(D){"use strict";var Ne;const Me=`
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
  background:none; box-shadow:none;
  display:grid; place-items:center;
  transition:transform .18s cubic-bezier(.2,.8,.2,1), box-shadow .18s ease;
  -webkit-tap-highlight-color:transparent;
}
/* วงกลมสีหลัก — วาดเฉพาะปุ่มแบบไอคอนในตัว และตอนเปิด (×) · ปุ่มรูปฟองแชทตอนปิดไม่มีวงกลมเลย
   (ไม่ใช้วิธีวาดแล้วค่อยซ่อน — บางเครื่องยังเห็นวงกลมโผล่ใต้ฟองแชท) */
.launcher:not([data-icon]),.launcher[data-open="true"]{
  /* แสงเงาบนสีหลัก — ให้ปุ่มดูนูน ไม่แบน (สีหลักตั้งจากคอนโซลได้) */
  background:
    radial-gradient(120% 90% at 30% 18%, rgba(255,255,255,.34), rgba(255,255,255,0) 58%),
    linear-gradient(160deg, rgba(255,255,255,0) 45%, rgba(0,0,0,.18)),
    linear-gradient(135deg, var(--accent-stops));
  box-shadow:
    inset 0 0 0 1px rgba(255,255,255,.22),
    0 2px 4px rgba(19,40,43,.12),
    0 10px 24px -6px var(--accent);
}
.launcher:hover{transform:translateY(-2px) scale(1.04)}
.launcher:not([data-icon]):hover,.launcher[data-open="true"]:hover{
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
.launcher[data-icon]:not([data-open="true"]){width:64px;height:64px;border-radius:0}
.launcher[data-icon]::after{display:none}
.licon{position:relative;display:block;width:64px;height:64px;filter:drop-shadow(0 6px 10px rgba(0,0,0,.28));animation:aio-float 3.2s ease-in-out infinite}
.licon img{width:100%;height:100%;border-radius:0;object-fit:contain;box-shadow:none}
.licon .tint{position:absolute;inset:0;background:linear-gradient(135deg, var(--accent-stops));mix-blend-mode:color;
  -webkit-mask:var(--licon) center/contain no-repeat;mask:var(--licon) center/contain no-repeat;pointer-events:none}
@keyframes aio-float{0%,100%{transform:translateY(0)}50%{transform:translateY(-3px)}}
@media (prefers-reduced-motion: reduce){.licon{animation:none}}
.ico{width:30px;height:30px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.ico .fill{fill:currentColor;stroke:none}
.launcher .close.ico{width:24px;height:24px;stroke-width:2.2}
/* × วางกลางปุ่มเสมอ — รูปฟองแชท (64px) ที่ซ่อนตอนเปิดยังกินช่อง grid ใหญ่กว่าปุ่ม (58px) ทำให้ × เยื้อง */
.launcher .close{position:absolute;inset:0;margin:auto}
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
/* ป้ายสถานะ — สีจาก tone ของตารางสถานะ (ok เขียว · wait เหลือง · bad แดง) */
.pill{display:inline-block;border-radius:999px;padding:1px 9px;font-size:11.5px;font-weight:600;white-space:nowrap;line-height:1.6}
.pill.t-ok{background:#e3f5ec;color:#146c43}
.pill.t-wait{background:#fdf1dc;color:#8a5300}
.pill.t-bad{background:#fde7e7;color:#a12626}
.datacard .dc-head{display:flex;justify-content:space-between;align-items:baseline;gap:8px;padding-bottom:6px;border-bottom:1px solid var(--line)}
.datacard .dc-head .dc-title{margin:0}
.datacard .dc-meta{font-size:11.5px;color:var(--muted);text-align:right}
/* stat — ตัวเลขเด่น */
.datacard .dc-statlabel{font-size:12px;color:var(--muted);font-weight:500}
.datacard .dc-statval{font-family:var(--font-mono);font-variant-numeric:tabular-nums;font-size:26px;font-weight:500;letter-spacing:-.3px;margin:2px 0 4px;word-break:break-word}
.datacard .dc-chips{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 4px}
.datacard .chip{background:var(--surface-2,#eef1f0);border-radius:999px;padding:2px 9px;font-size:11.5px;color:var(--ink-2)}
/* list — แถวละรายการ */
.datacard .dc-li{display:flex;align-items:center;gap:10px;padding:7px 0;border-bottom:1px dashed var(--line)}
.datacard .dc-li:last-of-type{border-bottom:0}
.datacard .dc-li-time{font-family:var(--font-mono);font-size:11.5px;color:var(--muted);min-width:44px;max-width:96px}
.datacard .dc-li-main{display:flex;flex-direction:column;flex:1 1 auto;min-width:0}
.datacard .dc-li-val{font-family:var(--font-mono);font-variant-numeric:tabular-nums;font-size:14px;font-weight:500;word-break:break-word}
.datacard .dc-li-sub{font-size:11.5px;color:var(--muted);word-break:break-word}
/* single — รายการเดียว */
.datacard .dc-single-head{display:flex;align-items:center;gap:8px;margin-bottom:8px}
.datacard .dc-single-head .dc-title{margin:0}
.datacard .dc-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 12px}
.datacard .dc-cell{display:flex;flex-direction:column;gap:1px;min-width:0}
.datacard .dc-cell span{font-size:11px;color:var(--muted)}
.datacard .dc-cell b{font-weight:500;font-size:13px;font-variant-numeric:tabular-nums;word-break:break-word}
/* ปุ่มถามต่อ — รองจากปุ่มสั่งหน้าเว็บ (.act) จึงเป็นขอบบาง ไม่ทึบ */
.sugs{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}
.sug{border:1px solid var(--accent);border-radius:999px;padding:5px 12px;font:inherit;font-size:12.5px;cursor:pointer;
  color:var(--accent);background:transparent;transition:background .15s ease}
.sug:hover{background:var(--accent-soft)}
.sug:focus-visible{outline:2px solid var(--accent-soft);outline-offset:2px}
.panel[data-bgimg] .sug{color:#e2e8f0;border-color:rgba(255,255,255,.35)}
.panel[data-bgimg] .sug:hover{background:rgba(255,255,255,.1)}
/* หน้าเว็บผู้เล่น: ไม่มีกรอบการ์ด — อ่านเป็นข้อความในฟองแชท (ไม่เหมือนการ์ดของหลังบ้าน) */
.datacard.inline{border:0;background:none;padding:0;margin-top:8px;color:inherit}
.datacard.inline .dc-title{font-size:13.5px;margin-bottom:6px}
.datacard.inline .dc-line{font-size:13px;padding:1px 0}
.datacard.inline .dc-item{padding:5px 0 5px 12px;position:relative;font-size:13px}
.datacard.inline .dc-item::before{content:"•";position:absolute;left:0;top:5px;opacity:.7}
.datacard.inline .dc-item+.dc-item{border-top:1px dashed currentColor;border-top-color:color-mix(in srgb, currentColor 22%, transparent)}
.datacard.inline .dc-sub{font-size:12px;opacity:.8;margin-top:1px}
.datacard.inline .dc-note{color:inherit;opacity:.75}
.datacard.inline .src{border:0;padding:0;margin-top:6px;opacity:.7;color:inherit}
.datacard.inline.k-error,.datacard.inline.k-denied{background:none}
/* ปุ่มใต้คำตอบ (หน้าเว็บผู้เล่น) — ไม่มีปุ่มแบบนี้ใน widget มาก่อน: เป็นปุ่มเดียวที่สั่งหน้าเว็บ จึงใช้สีไล่ของผู้ช่วยให้เด่นกว่าลิงก์ในการ์ด */
.acts{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}
.act{border:0;border-radius:999px;padding:7px 14px;font:inherit;font-size:13px;font-weight:600;cursor:pointer;
  color:var(--on-accent,#fff);background:linear-gradient(135deg, var(--accent-stops));box-shadow:0 2px 8px -2px var(--accent);
  transition:transform .15s ease, filter .15s ease}
.act:hover{transform:translateY(-1px);filter:brightness(1.06)}
.act:active{transform:scale(.97)}
.act:focus-visible{outline:2px solid var(--accent-soft);outline-offset:2px}
@media (prefers-reduced-motion: reduce){.act{transition:none}}
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
`;function Ue(e){const t=document.createElement("div");t.setAttribute("data-ai-office-host",""),t.style.all="initial",e?(getComputedStyle(e).position==="static"&&(e.style.position="relative"),t.style.position="absolute",t.style.inset="0",e.appendChild(t)):document.body.appendChild(t);const n=t.attachShadow({mode:"closed"}),a=document.createElement("style");return a.textContent=Me,n.appendChild(a),{host:t,root:n}}function Ge(e){const t=document.createElement("link");t.rel="stylesheet",t.href="https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap",e.appendChild(t)}const je=2e3;function We(e){const t=c("button","launcher");t.type="button",t.setAttribute("aria-label","เปิดผู้ช่วยหลังบ้าน"),t.setAttribute("aria-expanded","false"),t.dataset.open="false";const n=c("div","panel");n.dataset.open="false",n.setAttribute("role","dialog"),n.setAttribute("aria-label","ผู้ช่วยหลังบ้าน");const a=c("div","head"),o=c("div","avatar"),i=c("div","namewrap"),r=c("div","name"),s=c("div","tag");i.append(r,s);const f=c("div","site");f.title="เว็บที่กำลังคุยอยู่";const g=c("button","x hist");g.type="button",g.textContent="ประวัติ",g.title="ห้องแชทย้อนหลัง 7 วัน",g.setAttribute("aria-label","ห้องแชทย้อนหลัง 7 วัน");const p=c("button","x");p.type="button",p.textContent="×",p.setAttribute("aria-label","ปิด"),a.append(o,i,f,g,p);const l=c("div","log");l.setAttribute("aria-live","polite");const m=c("div","foot"),d=c("textarea");d.rows=1,d.maxLength=je,d.placeholder="พิมพ์คำถาม…",d.setAttribute("aria-label","คำถาม");const b=c("button","send");b.type="button",b.textContent="↑",b.setAttribute("aria-label","ส่ง"),m.append(d,b),n.append(a,l,m),e.append(t,n);const h={launcher:t,panel:n,head:{avatar:o,name:r,tag:s,site:f,history:g},log:l,input:d,send:b};return p.addEventListener("click",()=>X(h,!1)),h}function X(e,t){e.panel.dataset.open=String(t),e.launcher.dataset.open=String(t),e.launcher.setAttribute("aria-expanded",String(t)),e.launcher.setAttribute("aria-label",t?"ปิดผู้ช่วยหลังบ้าน":"เปิดผู้ช่วยหลังบ้าน")}const fe="http://www.w3.org/2000/svg";function Q(e,t){const n=document.createElementNS(fe,"svg");n.setAttribute("viewBox","0 0 24 24"),n.setAttribute("aria-hidden","true"),n.setAttribute("class",e);for(const a of t){const o=document.createElementNS(fe,"path");o.setAttribute("d",a.d),a.fill&&o.setAttribute("class","fill"),n.appendChild(o)}return n}const ge=[{d:"M3.5 6.25A3.25 3.25 0 0 1 6.75 3h10.5a3.25 3.25 0 0 1 3.25 3.25v7.5A3.25 3.25 0 0 1 17.25 17H11.5l-4.1 3.4c-.7.58-1.9.1-1.9-.83V17A3.25 3.25 0 0 1 3.5 13.75z"},{d:"M12 5.6l1.05 2.6 2.6 1.05-2.6 1.05L12 12.9l-1.05-2.6-2.6-1.05 2.6-1.05z",fill:!0}],Fe=[{d:"M7 7l10 10M17 7L7 17"}];function He(e,t){e.launcher.textContent="";const n=c("span","face");if(t){e.launcher.dataset.icon="true";const a=c("span","licon");a.style.setProperty("--licon",`url("${t}")`),a.append(xe(t),c("span","tint")),n.appendChild(a)}else delete e.launcher.dataset.icon,n.appendChild(Q("ico",ge));e.launcher.append(n,Q("ico close",Fe))}function _(e,t,n,a=""){const o=c("div","row "+t),i=c("div","bubble"+(a?" "+a:"")),r=c("div","txt");return r.textContent=n,i.appendChild(r),o.appendChild(i),e.appendChild(o),A(e),i}function z(e,t){const n=c("div","sys");return n.textContent=t,e.appendChild(n),A(e),n}function ee(e,t){const n=c("div","row ai"),a=c("div","bubble load"),o=c("span");o.textContent=t;const i=c("span","dots");return i.append(c("span"),c("span"),c("span")),a.append(o,i),n.appendChild(a),e.appendChild(n),A(e),{set:r=>o.textContent=r,remove:()=>n.remove()}}function he(e,t,n){const a=c("button","datacard room");a.type="button";const o=c("div","dc-title");if(o.textContent=e,a.appendChild(o),t){const i=c("div","src");i.textContent=t,a.appendChild(i)}return a.addEventListener("click",n),a}function te(e,t={}){var o,i,r,s,f,g;if(t.inline)return Xe(e);const n=e.kind==="ok"?e.style:void 0;if(n==="stat"||n==="list"||n==="single"){const p=c("div",`datacard s-${n} k-${j(e.kind)}`);if(n==="stat"?qe(p,e):n==="list"?Je(p,e):Ye(p,e),e.note){const l=c("div","dc-note");l.textContent=e.note,p.appendChild(l)}return p.appendChild(be(e)),p}const a=c("div","datacard k-"+j(e.kind));if(e.title){const p=c("div","dc-title");p.textContent=e.title,a.appendChild(p)}for(const p of(o=e.fields)!=null?o:[]){const l=c("div","dc-row"),m=c("span");m.textContent=p.label;const d=c("b");d.textContent=p.display,l.append(m,d),a.appendChild(l)}if(e.table&&((i=e.table.rows)!=null&&i.length)){const p=c("div","dc-tablewrap"),l=c("table","dc-table"),m=c("thead"),d=c("tr");for(const h of(r=e.table.columns)!=null?r:[]){const y=c("th");y.textContent=h.label,d.appendChild(y)}m.appendChild(d);const b=c("tbody");for(const h of e.table.rows){const y=c("tr");for(const[C,x]of h.entries()){const w=c("td");L((s=e.table.columns[C])==null?void 0:s.format)?w.appendChild(G((f=x==null?void 0:x.display)!=null?f:"",x==null?void 0:x.tone)):w.textContent=(g=x==null?void 0:x.display)!=null?g:"",y.appendChild(w)}b.appendChild(y)}l.append(m,b),p.appendChild(l),a.appendChild(p)}if(e.note){const p=c("div","dc-note");p.textContent=e.note,a.appendChild(p)}return a.appendChild(be(e)),a}function be(e){var o;const t=c("div","src"),n=c("span");n.textContent=e.kind==="reference"?"จากคู่มือของระบบ":"ข้อมูล ณ "+ae(e.fetched_at)+(e.cached?" · ค่าที่ดึงไว้ไม่เกิน 1 นาที":""),t.appendChild(n);const a=et((o=e.link)==null?void 0:o.path);if(e.link&&a){const i=c("a");i.href=a,i.textContent=(e.link.label||"เปิดหน้าจริง")+" →",i.target="_self",i.rel="noopener",t.appendChild(i)}return t}const L=e=>!!e&&e.startsWith("status:"),Ve=e=>e==="money";function G(e,t){const n=c("span","pill t-"+j(t!=null?t:"wait"));return n.textContent=e,n}function Ke(e,t,n=""){const a=c("div","dc-head"),o=c("span","dc-title");if(o.textContent=t.title,a.appendChild(o),n){const i=c("span","dc-meta");i.textContent=n,a.appendChild(i)}e.appendChild(a)}function qe(e,t){var i,r,s;const[n,...a]=(i=t.fields)!=null?i:[],o=c("div","dc-statlabel");if(o.textContent=n?`${n.label} · ${t.title}`:t.title,e.appendChild(o),n){const f=c("div","dc-statval");f.textContent=n.display,e.appendChild(f)}if(a.length){const f=c("div","dc-chips");for(const g of a){if(L(g.format)){f.appendChild(G(g.display,g.tone));continue}const p=c("span","chip");p.textContent=`${g.label} ${g.display}`,f.appendChild(p)}e.appendChild(f)}(s=(r=t.table)==null?void 0:r.rows)!=null&&s.length&&ne(e,t)}function Je(e,t){var n;Ke(e,t,((n=t.fields)!=null?n:[]).map(a=>`${a.label} ${a.display}`).join(" · ")),ne(e,t)}function ne(e,t){var r,s,f,g,p,l,m,d;const n=(s=(r=t.table)==null?void 0:r.columns)!=null?s:[],a=n.findIndex(b=>b.format==="datetime"||b.format==="time"||b.format==="date"),o=n.findIndex(b=>Ve(b.format)),i=o>=0?o:n.findIndex((b,h)=>{var y;return h!==a&&!L((y=n[h])==null?void 0:y.format)});for(const b of(g=(f=t.table)==null?void 0:f.rows)!=null?g:[]){const h=c("div","dc-li");if(a>=0){const v=c("span","dc-li-time");v.textContent=(l=(p=b[a])==null?void 0:p.display)!=null?l:"",h.appendChild(v)}const y=c("div","dc-li-main"),C=c("span","dc-li-val");C.textContent=i>=0&&(d=(m=b[i])==null?void 0:m.display)!=null?d:"",y.appendChild(C);const x=[];let w=null;if(b.forEach((v,k)=>{var S,T,Pe;k===a||k===i||!(v!=null&&v.display)||(L((S=n[k])==null?void 0:S.format)?w=G(v.display,v.tone):x.push(`${(Pe=(T=n[k])==null?void 0:T.label)!=null?Pe:""} ${v.display}`.trim()))}),x.length){const v=c("span","dc-li-sub");v.textContent=x.join(" · "),y.appendChild(v)}h.appendChild(y),w&&h.appendChild(w),e.appendChild(h)}}function Ye(e,t){var g,p,l,m,d,b,h,y,C;const n=(p=(g=t.table)==null?void 0:g.columns)!=null?p:[],a=(d=(m=(l=t.table)==null?void 0:l.rows)==null?void 0:m[0])!=null?d:[],o=[...(b=t.fields)!=null?b:[],...a.map((x,w)=>{var v,k,S,T;return{label:(k=(v=n[w])==null?void 0:v.label)!=null?k:"",display:(S=x==null?void 0:x.display)!=null?S:"",format:(T=n[w])==null?void 0:T.format,tone:x==null?void 0:x.tone}})],i=o.find(x=>L(x.format)),r=c("div","dc-single-head");i&&r.appendChild(G(i.display,i.tone));const s=c("span","dc-title");s.textContent=t.title,r.appendChild(s),e.appendChild(r);const f=c("div","dc-grid");for(const x of o){if(x===i||!x.display)continue;const w=c("div","dc-cell"),v=c("span");v.textContent=x.label;const k=c("b");k.textContent=x.display,w.append(v,k),f.appendChild(w)}e.appendChild(f),((C=(y=(h=t.table)==null?void 0:h.rows)==null?void 0:y.length)!=null?C:0)>1&&ne(e,{...t,table:{columns:n,rows:t.table.rows.slice(1)}})}function Ze(e,t){const n=c("button","sug");return n.type="button",n.textContent=e,n.addEventListener("click",t),n}function Xe(e){var o,i,r,s,f,g,p;const t=c("div","datacard inline k-"+j(e.kind));if(e.title){const l=c("div","dc-title");l.textContent=e.title,t.appendChild(l)}for(const l of(o=e.fields)!=null?o:[]){const m=c("div","dc-line");m.textContent=`${l.label}: ${l.display}`,t.appendChild(m)}const n=(r=(i=e.table)==null?void 0:i.columns)!=null?r:[];for(const l of(f=(s=e.table)==null?void 0:s.rows)!=null?f:[]){const m=c("div","dc-item"),d=c("b");d.textContent=(p=(g=l[0])==null?void 0:g.display)!=null?p:"",m.appendChild(d);const b=l.slice(1).map((h,y)=>{var C,x;return h!=null&&h.display?`${(x=(C=n[y+1])==null?void 0:C.label)!=null?x:""} ${h.display}`.trim():""}).filter(Boolean);if(b.length){const h=c("div","dc-sub");h.textContent=b.join(" · "),m.appendChild(h)}t.appendChild(m)}if(e.note){const l=c("div","dc-note");l.textContent=e.note,t.appendChild(l)}const a=c("div","src");return a.textContent=e.kind==="reference"?"จากคู่มือของเว็บ":"ข้อมูลจากระบบจริง "+ae(e.fetched_at),t.appendChild(a),t}function Qe(e,t){const n=c("button","act");return n.type="button",n.textContent=e,n.addEventListener("click",t),n}function ae(e){const t=new Date(e);if(isNaN(t.getTime()))return"-";const n={timeZone:"Asia/Bangkok"};try{const a=t.toLocaleTimeString("th-TH",{...n,hour:"2-digit",minute:"2-digit",hour12:!1}),o=t.toLocaleDateString("en-CA",n),i=new Date().toLocaleDateString("en-CA",n);if(o===i)return a;const[,r,s]=o.split("-");return`${s}/${r} ${a}`}catch{return t.toISOString().slice(0,16).replace("T"," ")}}function et(e){if(!e)return"";const t=e.trim();return t.startsWith("#")?t:!t.startsWith("/")||t.startsWith("//")||t.includes("\\")?"":t}function tt(e,t){const{position:n,offset_x:a,offset_y:o}=t.placement,i=n==="bottom-left";for(const r of[e.launcher,e.panel])r.style.left="",r.style.right="";e.launcher.style.bottom=O(o),e.panel.style.bottom=O(o+68),i?(e.launcher.style.left=O(a),e.panel.style.left=O(a)):(e.launcher.style.right=O(a),e.panel.style.right=O(a))}function nt(e,t,n="",a=!0){var i,r;e.head.name.textContent=t.display_name||(a?"ผู้ช่วยหลังบ้าน":"ผู้ช่วย"),e.head.tag.textContent=(i=t.tagline)!=null?i:"",e.head.site.textContent=a&&(t.service_label||t.service_id)||"",e.head.site.style.display=e.head.site.textContent?"":"none";const o=oe(t.avatar_url,n);e.head.avatar.textContent="",o?e.head.avatar.appendChild(xe(o)):e.head.avatar.appendChild(Q("ico",ge)),He(e,oe(t.launcher_icon,n)),o||t.tagline||t.background?e.panel.dataset.rich="true":delete e.panel.dataset.rich,it(e.log,(r=t.background)!=null?r:"",n),e.log.dataset.bg==="image"?e.panel.dataset.bgimg="true":delete e.panel.dataset.bgimg}const at=/^(https:\/\/|http:\/\/localhost(:\d+)?\/)[A-Za-z0-9._~:/?#[\]@!$&*+,;=%-]+$/,ot=new Set(["dots","grid","diagonal","glow"]);function oe(e,t){if(!e)return"";if(e.startsWith("asset:")){const n=e.slice(6);return/^[a-z0-9_-]+\/[A-Za-z0-9._-]+$/.test(n)?`${t}/widget/v1/assets/${n}`:""}return at.test(e)?e:""}function it(e,t,n){if(e.style.removeProperty("--log-img"),delete e.dataset.bg,t.startsWith("pattern:")){const o=t.slice(8);ot.has(o)&&(e.dataset.bg=o);return}const a=oe(t,n);a&&(e.dataset.bg="image",e.style.setProperty("--log-img",`url("${a}")`))}const me=/^#[0-9a-f]{6}$/i;function ie(e,t,n,a=""){const o=["--accent","--accent-stops","--accent-soft","--on-accent"],i=(Array.isArray(t)?t:t?[t]:[]).filter(d=>me.test(d)).slice(0,4);if(!i.length){for(const d of o)e.style.removeProperty(d);return}const r=i[0];i.length===1&&i.push(r);const s=i.map(d=>[1,3,5].map(b=>parseInt(d.slice(b,b+2),16))),f=s[0],g=[0,1,2].map(d=>s.reduce((b,h)=>b+h[d],0)/s.length),p=n==="dark"?[21,33,35]:[255,255,255],l=d=>"#"+f.map((b,h)=>Math.round(b*d+p[h]*(1-d)).toString(16).padStart(2,"0")).join(""),m=g.map(d=>d/255).map(d=>d<=.03928?d/12.92:((d+.055)/1.055)**2.4).reduce((d,b,h)=>d+b*[.2126,.7152,.0722][h],0);e.style.setProperty("--accent",r),e.style.setProperty("--accent-stops",i.join(", ")),e.style.setProperty("--accent-soft",l(n==="dark"?.25:.16)),e.style.setProperty("--on-accent",me.test(a)?a:m>.45?"#13282B":"#ffffff")}function rt(e){if(!e)return null;const t=getComputedStyle(document.documentElement),n=document.createElement("span");n.style.display="none",document.body.appendChild(n);const a=i=>{if(!i||!/^--[A-Za-z0-9_-]+$/.test(i))return"";const r=t.getPropertyValue(i).trim();if(!r||(n.style.color="",n.style.color=r,!n.style.color))return"";const s=/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?/.exec(getComputedStyle(n).color);return!s||s[4]!==void 0&&Number(s[4])===0?"":"#"+[s[1],s[2],s[3]].map(f=>Number(f).toString(16).padStart(2,"0")).join("")},o={accent:a(e.accent),accent2:a(e.accent_2),on:a(e.on_accent)};return n.remove(),o.accent?o:null}function A(e){e.scrollTop=e.scrollHeight}function xe(e){const t=document.createElement("img");return t.src=e,t.alt="",t.referrerPolicy="no-referrer",t}function j(e){return String(e||"").replace(/[^a-z_]/g,"")}function O(e){return`${e|0}px`}function c(e,t){const n=document.createElement(e);return t&&(n.className=t),n}const st=["avatar_url","display_name","greeting","tagline","launcher_icon","background","theme","accent_color","accent_colors","color_source","placement","service_label","is_hidden"],ct=2*1024*1024;function lt(e,t,n){if(!e||typeof t!="string"||!t.startsWith("/")||t.startsWith("//")||t.includes("\\")||/^[a-z][a-z0-9+.-]*:/i.test(t)||t.includes("://"))return null;let a;try{a=new URL(e)}catch{return null}const o=a.pathname.replace(/\/+$/,"");let i;try{i=new URL(a.origin+o+t)}catch{return null}if(i.origin!==a.origin||!(i.pathname===o||i.pathname.startsWith(o+"/")))return null;if(n)for(const[r,s]of Object.entries(n))i.searchParams.set(r,String(s));return i.toString()}async function ye(e,t,n,a="Bearer"){var f;if(!n)return{status:401,body:""};const o=String(t.method||"").toUpperCase();if(o!=="GET"&&o!=="POST")return{status:0,body:""};const i=lt(e,t.path,t.query);if(!i)return{status:0,body:""};const r=new AbortController,s=setTimeout(()=>r.abort(),Math.max(1e3,(f=t.timeout_ms)!=null?f:2e4));try{const g={method:o,credentials:"omit",signal:r.signal,headers:{Authorization:`${a} ${n}`,Accept:"application/json"}};o==="POST"&&t.body!==void 0&&t.body!==null&&(g.headers["Content-Type"]="application/json",g.body=JSON.stringify(t.body));const p=await fetch(i,g),l=await p.text();return l.length>ct?{status:0,body:""}:{status:p.status,body:l}}catch{return{status:0,body:""}}finally{clearTimeout(s)}}const N="ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง";class E extends Error{constructor(t,n=""){super(t),this.code=n}}const dt={SERVICE_DISABLED:"ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่",CHAT_DISABLED:"ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่",LLM_NOT_CONFIGURED:"ผู้ช่วยยังไม่พร้อมใช้งาน — ผู้ดูแลระบบยังตั้งค่าไม่เสร็จ",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์ใช้ผู้ช่วยในเว็บนี้",SESSION_EXPIRED:"การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่",NOT_AUTHENTICATED:"การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่",BACKOFFICE_UNAVAILABLE:"ตรวจสอบสิทธิ์กับหลังบ้านไม่ได้ชั่วคราว กรุณาลองใหม่อีกครั้ง",TICKET_INVALID:"การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า",TICKET_SERVICE_MISMATCH:"การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า",ORIGIN_NOT_REGISTERED:"ผู้ช่วยยังไม่ได้เปิดให้หลังบ้านนี้"},pt=new Set(["SERVICE_DISABLED","CHAT_DISABLED","SERVICE_NOT_ALLOWED","ORIGIN_NOT_REGISTERED"]),re=new Set(["TICKET_INVALID","TICKET_SERVICE_MISMATCH"]);async function P(e,t){var o,i;const n=await e.json().catch(()=>null),a=(o=n==null?void 0:n.message)!=null?o:"";return new E((i=dt[a])!=null?i:t,a)}async function ut(e){const{apiBase:t,service:n,session:a,on:o}=e,i=`${t}/api/ai/widget/service/${encodeURIComponent(n)}`;let r=e.conversationID;const s=async d=>fetch(`${i}/chat`,{method:"POST",headers:{Authorization:`Bearer ${d}`,"Content-Type":"application/json"},body:JSON.stringify({conversation_id:r,text:e.text}),signal:e.signal});let f=await a.ticket(n),g=await s(f);if(!g.ok||!g.body){let d=await P(g,N);if(re.has(d.code)&&(a.invalidate(),f=await a.ticket(n),g=await s(f),d=g.ok&&g.body?null:await P(g,N)),d)throw re.has(d.code)&&a.invalidate(),d}const p=[],l=d=>{var b;return!((b=e.signal)!=null&&b.aborted)&&p.push(ye(a.hostApiBase(),d,a.readToken(),a.authScheme()).then(h=>fetch(`${i}/chat/relay/${encodeURIComponent(d.id)}`,{method:"POST",headers:{Authorization:`Bearer ${f}`,"Content-Type":"application/json"},body:JSON.stringify(h)})).then(()=>{}).catch(()=>{}))};let m=null;if(await we(g.body,(d,b)=>{var y,C,x,w,v,k;const h=b;switch(d){case"status":typeof h.conversation_id=="string"&&(r=h.conversation_id),o.status(String((y=h.text)!=null?y:""));break;case"fetch":l(h);break;case"card":o.card(h);break;case"suggest":typeof h.label=="string"&&typeof h.ask=="string"&&((C=o.suggest)==null||C.call(o,{label:h.label,ask:h.ask}));break;case"action":typeof h.id=="string"&&typeof h.label=="string"&&((x=o.action)==null||x.call(o,{id:h.id,label:h.label}));break;case"token":o.token(String((w=h.text)!=null?w:""));break;case"error":m=new E(String((v=h.message)!=null?v:"ผู้ช่วยตอบไม่สำเร็จ"),String((k=h.code)!=null?k:""));break}}),await Promise.all(p),m)throw m;return r}async function ve(e,t,n,a){const o=`${e}/api/ai/widget/service/${encodeURIComponent(t)}${a}`,i=async()=>fetch(o,{headers:{Authorization:`Bearer ${await n.ticket(t)}`}});let r=await i();if(!r.ok){let f=await P(r,N);if(re.has(f.code)&&(n.invalidate(),r=await i(),f=r.ok?null:await P(r,N)),f)throw f}const s=await r.json().catch(()=>null);if(!(s!=null&&s.payload))throw new E(N);return s.payload}function ft(e,t,n,a=7){return ve(e,t,n,`/conversations?days=${a}`)}function gt(e,t,n,a){return ve(e,t,n,`/conversations/${encodeURIComponent(a)}`)}async function ht(e){const t=await fetch(`${e.apiBase}/api/ai/admin/assistant`,{method:"POST",headers:{Authorization:`Bearer ${e.token}`,"Content-Type":"application/json"},body:JSON.stringify({messages:e.messages,page:e.page})});if(!t.ok||!t.body){const a=await t.json().catch(()=>null);throw new E((a==null?void 0:a.error)||`เซิร์ฟเวอร์ตอบ ${t.status}`)}let n=null;if(await we(t.body,(a,o)=>{var r,s,f;const i=o;a==="status"?e.on.status(String((r=i.text)!=null?r:"")):a==="card"?e.on.card(i):a==="token"?e.on.token(String((s=i.text)!=null?s:"")):a==="error"&&(n=new E(String((f=i.message)!=null?f:"ผู้ช่วยตอบไม่สำเร็จ")))}),n)throw n}async function we(e,t){const n=e.getReader(),a=new TextDecoder;let o="";for(;;){const{value:i,done:r}=await n.read();if(r)break;o+=a.decode(i,{stream:!0});let s;for(;(s=o.indexOf(`

`))>=0;){const f=o.slice(0,s);o=o.slice(s+2);let g="message",p="";for(const l of f.split(`
`))l.startsWith("event:")?g=l.slice(6).trim():l.startsWith("data:")&&(p+=l.slice(5).trim());p&&t(g,JSON.parse(p))}}}const B=new Set(["","null","undefined","false"]);function se(e){try{if(e==="localStorage")return localStorage;if(e==="sessionStorage")return sessionStorage}catch{}return null}function Ce(e){var t,n;if(!e)return"";try{const a=(n=(t=se(e.source))==null?void 0:t.getItem(bt(e)))!=null?n:"";if(B.has(a.trim()))return"";if(e.format!=="json-expiration")return ke(a);const o=JSON.parse(a),i=o==null?void 0:o[e.value_field||"value"];if(typeof i!="string"||B.has(i))return"";const r=o[e.expiration_field||"expiration"];return typeof r=="number"&&r<=Math.floor(Date.now()/1e3)?"":ke(i)}catch{return""}}function bt(e){var n,a,o;if(!e.key_from)return e.key;const t=((a=(n=se(e.key_from.source))==null?void 0:n.getItem(e.key_from.key))!=null?a:"").trim();return e.key.split("{key_from}").join(B.has(t)?(o=e.key_from.default)!=null?o:"":t)}function ke(e){const t=e.trim(),n=/^bearer(\s+|$)/i.exec(t),a=n?t.slice(n[0].length).trim():t;return B.has(a)?"":a}function ce(e){var t,n,a,o,i;if(!e)return"";if(e.source==="office")return(t=e.value)!=null?t:"";try{let r="";if(e.source==="query"){if(r=(n=new URLSearchParams(location.search).get(e.key))!=null?n:"",!r){const s=location.hash.indexOf("?");s>=0&&(r=(a=new URLSearchParams(location.hash.slice(s+1)).get(e.key))!=null?a:"")}}else r=(i=(o=se(e.source))==null?void 0:o.getItem(e.key))!=null?i:"";if(r=r.trim(),B.has(r))return"";if(e.encoding==="base64"){const s=atob(r).trim();return B.has(s)?"":s}return r}catch{return""}}function _e(e){return e?e.source==="office"?"service ของ domain นี้ (เปิดอย่างน้อย 1 service ที่คอนโซล)":e.source==="query"?`?${e.key}=`:`${e.source}["${e.key}"]`:"(ไม่ทราบ)"}function mt(e){const t=e.split(".")[1];if(!t)return null;try{const n=t.replace(/-/g,"+").replace(/_/g,"/"),a=atob(n+"=".repeat((4-n.length%4)%4)),o=Uint8Array.from(a,i=>i.charCodeAt(0));return JSON.parse(new TextDecoder().decode(o))}catch{return null}}function Se(e,t){const n=W(e,t.path);if(!Array.isArray(n))return[];const a=[];for(const o of n){if(t.where_field&&W(o,t.where_field)!==t.where_value)continue;const i=t.pluck?W(o,t.pluck):o;typeof i=="string"&&i&&a.push(i)}return a}function W(e,t){let n=e;if(!t)return n;for(const a of t.split(".")){if(!n||typeof n!="object")return;n=n[a]}return n}const Ee="ai-office:guest_id";let F="";function xt(){try{const e=localStorage.getItem(Ee);if(e&&/^[A-Za-z0-9_-]{16,64}$/.test(e))return e}catch{}if(!F){const e=new Uint8Array(18);crypto.getRandomValues(e),F=btoa(String.fromCharCode(...e)).replace(/\+/g,"-").replace(/\//g,"_").replace(/=+$/,"")}try{localStorage.setItem(Ee,F)}catch{}return F}const yt=180*1e3;class vt{constructor(t,n=""){this.apiBase=t,this.hostAPIBase=n,this.cfg=null,this.loading=null,this.cur=null}hostApiBase(){var t;return this.hostAPIBase||((t=this.cfg)==null?void 0:t.host_api_base)||""}readToken(){var t;return Ce((t=this.cfg)==null?void 0:t.token)}credential(){var n;const t=this.readToken();return t?`Bearer ${t}`:(n=this.cfg)!=null&&n.guest?`Guest ${xt()}`:""}isGuest(){return this.credential().startsWith("Guest ")}authScheme(){var t;return((t=this.cfg)==null?void 0:t.auth_scheme)||"Bearer"}async pageConfig(){var t;return this.cfg?this.cfg:((t=this.loading)!=null||(this.loading=(async()=>{var o;const n=await fetch(`${this.apiBase}/api/ai/widget/page-config`),a=await n.json().catch(()=>null);if(!n.ok||!(a!=null&&a.payload))throw new Ae((o=a==null?void 0:a.message)!=null?o:String(n.status));return this.cfg=a.payload,this.cfg})().finally(()=>{this.loading=null})),this.loading)}async ticket(t){var s;const n=await this.pageConfig(),a=this.credential();if(this.cur&&this.cur.service===t&&this.cur.cred===a&&this.cur.expiresAt-Date.now()>yt)return this.cur.ticket;const o=a.startsWith("Guest ")?[]:await this.readPermissions(n,t),i=await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(t)}/browser-session`,{method:"POST",headers:{Authorization:a,"Content-Type":"application/json"},body:JSON.stringify({permissions:o})});if(!i.ok)throw await P(i,`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${i.status})`);const r=await i.json().catch(()=>null);if(!((s=r==null?void 0:r.payload)!=null&&s.ticket))throw new E(`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${i.status})`);return this.cur={service:t,cred:a,ticket:r.payload.ticket,expiresAt:Date.now()+r.payload.expires_in*1e3},this.cur.ticket}invalidate(){this.cur=null}async readPermissions(t,n){const a=t.identity,o=a==null?void 0:a.permissions;if(!(o!=null&&o.path))return[];if(a!=null&&a.permissions_token){const s=Ce(a.permissions_token);return s?Se(W(mt(s),a.root),o):[]}const i=a==null?void 0:a.permissions_request;if(!i)return[];const r=await ye(this.hostApiBase(),{id:"perm",method:"GET",path:i.split("{service}").join(encodeURIComponent(n))},this.readToken(),this.authScheme());if(r.status!==200)return console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${r.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`),[];try{return Se(JSON.parse(r.body),o)}catch{return[]}}}class Ae extends Error{}function Ie(e){try{return document.querySelector(e)}catch{return null}}function wt(e){return e.open.click?Ie(e.open.click)!==null:!0}function Ct(e){const t=e.open,n=window.$nuxt;try{if(t.bv_modal)return n!=null&&n.$bvModal?(n.$bvModal.show(t.bv_modal),!0):!1;if(t.click){const a=Ie(t.click);return a?(a.click(),!0):!1}if(t.path&&t.path.startsWith("/")&&!t.path.startsWith("//"))return n!=null&&n.$router?Promise.resolve(n.$router.push(t.path)).catch(()=>{}):location.assign(t.path),!0}catch{}return!1}const kt=["serviceId","websiteId","businessId","tenant","tenantId","officeId","apiKey"],le=new Map;function _t(){le.clear()}function Te(e,t=""){const n=e+"|"+t;let a=le.get(n);return a||(a=new vt(e,t),le.set(n,a)),a}const H={enabled:!0,office_id:"",service_id:"",service_label:"",is_hidden:!1,avatar_url:"",display_name:"ผู้ช่วยหลังบ้าน",greeting:"สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",theme:"auto",placement:{position:"bottom-right",offset_x:12,offset_y:12}},St=20;let u=null;function V(){var e;window.removeEventListener("message",$e),Z(),(e=u==null?void 0:u.abort)==null||e.abort(),u==null||u.host.remove(),u=null}async function K(e={}){var m,d,b,h,y,C,x,w,v,k;V();const t=(m=e.dataset)!=null?m:{};for(const S of kt)t[S]&&(console.warn(`[ai-office] ไม่รับ data-${Rt(S)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`),delete t[S]);const n=(d=t.previewMount)!=null?d:"",a=n!=="",o=(h=(b=e.apiBase)!=null?b:t.apiBase)!=null?h:"",i=Te(o,t.hostApiBase);let r;if(e.console)r={...H,display_name:(y=e.console.displayName)!=null?y:"ผู้ช่วย AI Office",greeting:(C=e.console.greeting)!=null?C:"สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)",service_label:(x=e.console.label)!=null?x:"คอนโซล",placement:{position:"bottom-right",offset_x:20,offset_y:20}};else if(a)r={...H,...(w=e.bootstrap)!=null?w:{}};else if(e.bootstrap)r={...H,...e.bootstrap};else{const S=await Et(o,i,e.onFetch);if(!S)return;r={...H,...S}}if(!a&&!r.enabled)return;const s=a?document.querySelector(n):null;if(a&&!s){console.warn(`[ai-office] ไม่พบกล่อง preview: ${n}`);return}const{host:f,root:g}=Ue(s);a&&f.setAttribute("data-preview","true"),Ge(g);const p=We(g),l=!a&&!e.console&&!e.bootstrap?await i.pageConfig().catch(()=>null):null;u={ui:p,host:f,cfg:r,preview:a,apiBase:o,session:i,conversationID:"",busy:!1,console:(v=e.console)!=null?v:null,history:[],abort:null,listing:!1,showSite:(l==null?void 0:l.audience)!=="player",pageColors:l==null?void 0:l.page_colors,pageActions:(k=l==null?void 0:l.page_actions)!=null?k:[],inlineCards:(l==null?void 0:l.audience)==="player"},(a||e.console)&&(p.head.history.style.display="none"),q(r),p.launcher.addEventListener("click",()=>$()),p.head.history.addEventListener("click",()=>{De()}),p.send.addEventListener("click",()=>M(e)),p.input.addEventListener("keydown",S=>{const T=S;T.key==="Enter"&&!T.shiftKey&&!T.isComposing&&(T.preventDefault(),M(e))}),a&&(X(p,!0),window.addEventListener("message",$e)),e.console||zt()}function q(e){if(!u)return;u.cfg=e;const t=ze(e.theme);u.host.setAttribute("data-theme",t);const n=pe(t);nt(u.ui,e,u.apiBase,u.showSite),tt(u.ui,e),u.ui.launcher.style.display=e.is_hidden?"none":"grid",u.ui.log.textContent="",e.greeting&&_(u.ui.log,"ai",e.greeting),u.preview&&z(u.ui.log,I.preview),n||Tt()}function $e(e){if(e.origin!==window.location.origin)return;const t=e.data;if(!t||t.type!=="ai-office:preview-config"||!t.config||!u||!u.preview)return;const n={};for(const a of st)a in t.config&&(n[a]=t.config[a]);q({...u.cfg,...n})}async function Et(e,t,n){var s,f,g,p,l;const a=await de(t);if(!a)return null;const o=t.credential(),i=ce(a.service);if(!o||!i){const m=_e(a.token),d=_e(a.service);return R(!o&&!i?`ไม่พบ ${m} และ ${d} — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน`:o?`ไม่พบ ${d} — ยังไม่ได้เลือกเว็บในหลังบ้าน (หน้านี้ไม่ได้ผูกกับเว็บใด)`:`ไม่พบ ${m} ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว`),null}const r=`${e}/api/ai/widget/service/${encodeURIComponent(i)}/bootstrap`;n==null||n({url:r});try{const m=await fetch(r,{headers:{Authorization:o}}),d=await m.json().catch(()=>null);if(!m.ok)return R(`เซิร์ฟเวอร์ตอบ ${m.status} ${(s=d==null?void 0:d.message)!=null?s:""} — ${(f=d==null?void 0:d.error)!=null?f:""}`.trim(),J[(g=d==null?void 0:d.message)!=null?g:""]),null;const b=(p=d==null?void 0:d.payload)!=null?p:null;if(!b)return null;if(!b.enabled)return R(`ยังไม่เปิดใช้งาน (reason: ${b.reason})`,J[(l=b.reason)!=null?l:""]),b;try{await t.ticket(i)}catch(h){const y=h instanceof E?h.code:"";return R(`ขอตั๋วแชทไม่สำเร็จ (${y||h.message})`,J[y]),null}return b}catch(m){return R(`เรียก ${r} ไม่สำเร็จ — ${m.message}`,"หลังบ้าน ai ทำงานอยู่ไหม"),null}}async function de(e){var t;try{return await e.pageConfig()}catch(n){const a=n instanceof Ae?n.message:"";return R(`โหลดการตั้งค่าของหน้านี้ไม่สำเร็จ (${a||n.message})`,(t=J[a])!=null?t:"หลังบ้าน ai ทำงานอยู่ไหม"),null}}function R(e,t){console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${e}`+(t?`
           → ${t}`:""))}const J={ORIGIN_NOT_REGISTERED:`โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,ORIGIN_REQUIRED:"เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า",SERVICE_NOT_ALLOWED:"บัญชีนี้ไม่มีสิทธิ์เปิดเว็บนี้ในหลังบ้าน (รายชื่อเว็บของบัญชี)",KIND_NOT_SUPPORTED:'ชนิดหลังบ้านของ domain นี้ยังไม่มี connector — ตรวจ "ชนิดหลังบ้าน" ที่คอนโซล',SESSION_EXPIRED:"token หมดอายุ ให้ล็อกอินหลังบ้านใหม่",NOT_AUTHENTICATED:"ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้",BACKOFFICE_UNAVAILABLE:"ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว",office_disabled:"office นี้ถูกปิดทั้งชุดที่คอนโซล",service_disabled:"service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้",not_in_allowlist:"เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล",wrong_office:"token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้",no_service:"ยังไม่ได้เลือกเว็บในหลังบ้าน"};function pe(e=ze((t=>(t=u==null?void 0:u.cfg.theme)!=null?t:"auto")())){var o;if(!u)return!0;const n=u.cfg,a=n.color_source==="site"?rt(u.pageColors):null;return a?ie(u.host,[a.accent,a.accent2||a.accent],e,a.on):n.color_source==="site"?ie(u.host,void 0,e):ie(u.host,(o=n.accent_colors)!=null&&o.length?n.accent_colors:n.accent_color,e),n.color_source!=="site"||!!a}const At=4e3,It=2e4;let Y=null;function Tt(){Z();const e=u;if(!e||e.preview)return;const t=Date.now();e.ui.launcher.style.visibility="hidden",Y=setInterval(()=>{if(u!==e)return Z();const n=pe(),a=Date.now()-t;(n||a>At)&&(e.ui.launcher.style.visibility=""),(n||a>It)&&Z()},250)}function Z(){Y&&clearInterval(Y),Y=null}function $(e){if(!u)return;const t=e!=null?e:u.ui.panel.dataset.open!=="true";t&&pe(),X(u.ui,t),t&&u.ui.input.focus()}const I={preview:"โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง",unavailable:"ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง",empty:"(ไม่มีคำตอบ)",rooms:"ห้องแชทย้อนหลัง 7 วัน — แตะเพื่อเปิดคุยต่อ",noRooms:"ยังไม่มีห้องแชทใน 7 วันนี้ · ห้องจากการล็อกอินครั้งก่อนเปิดจากที่นี่ไม่ได้ (เพื่อความปลอดภัย)"};async function M(e){var p;const t=u;if(!t||t.busy||t.ui.input.disabled)return;const n=t.ui.input.value.trim();if(!n)return;if(t.ui.input.value="",_(t.ui.log,"me",n),t.preview){_(t.ui.log,"ai",I.preview);return}if(t.console){await Bt(t,t.console,n);return}const a=await ue(t);if(!a){_(t.ui.log,"ai",I.unavailable,"err");return}t.listing&&(t.listing=!1,t.conversationID="",q(t.cfg),_(t.ui.log,"me",n)),(p=e.onFetch)==null||p.call(e,{url:`${t.apiBase}/api/ai/widget/service/${encodeURIComponent(a)}/chat`,body:{text:n}}),t.busy=!0,t.ui.send.disabled=!0,t.ui.head.history.disabled=!0;const o=t.abort=new AbortController,i=ee(t.ui.log,"กำลังส่งคำถาม…");let r=null,s=null,f=null;const g=()=>{if(r)return;const l=_(t.ui.log,"ai","");r=l.querySelector(".txt"),s=c("div","cards"),f=c("div","acts"),l.append(s,f)};try{t.conversationID=await ut({apiBase:t.apiBase,service:a,session:t.session,conversationID:t.conversationID,text:n,signal:o.signal,on:{status:l=>i.set(l||"กำลังทำงาน…"),card:l=>{g(),s.appendChild(te(l,{inline:t.inlineCards})),A(t.ui.log)},action:l=>{g(),Re(t,f,l),A(t.ui.log)},suggest:l=>{g(),Be(t,f.parentElement,l,e),A(t.ui.log)},token:l=>{var m;i.remove(),g(),r.textContent=((m=r.textContent)!=null?m:"")+l,A(t.ui.log)}}}),r||_(t.ui.log,"ai",I.empty)}catch(l){if(o.signal.aborted)return;const m=l instanceof E?l.message:I.unavailable;_(t.ui.log,"ai",m,"err");const d=l instanceof E?l.code:"";d==="not_found"&&(t.conversationID=""),pt.has(d)&&Ot(t)}finally{i.remove(),t.abort=null,t.busy=!1,t.ui.send.disabled=t.ui.input.disabled,t.ui.head.history.disabled=!1}}async function ue(e){const t=await de(e.session);return t?ce(t.service):""}async function De(){const e=u;if(!e||e.busy||e.preview||e.console)return;const t=await ue(e);if(!t)return;e.busy=!0,e.ui.head.history.disabled=!0;const n=ee(e.ui.log,"กำลังโหลดประวัติ…");try{const a=await ft(e.apiBase,t,e.session,7);if(u!==e)return;e.listing=!0,e.ui.log.textContent="",z(e.ui.log,I.rooms),e.ui.log.appendChild(he("＋ เริ่มห้องใหม่","",()=>$t(e)));for(const o of a)e.ui.log.appendChild(he(o.title||"(ไม่มีหัวข้อ)","คุยล่าสุด "+ae(o.updated_at),()=>{Oe(e,t,o.id)}));a.length||z(e.ui.log,I.noRooms),A(e.ui.log)}catch(a){_(e.ui.log,"ai",a instanceof E?a.message:I.unavailable,"err")}finally{n.remove(),e.busy=!1,e.ui.head.history.disabled=!1}}function $t(e){e.listing=!1,e.conversationID="",q(e.cfg),e.ui.input.focus()}async function Oe(e,t,n){if(!e.busy){e.busy=!0;try{const a=await gt(e.apiBase,t,e.session,n);if(u!==e)return;e.listing=!1,e.conversationID=a.id,e.ui.log.textContent="",z(e.ui.log,`ห้องเดิม: ${a.title} · ตัวเลขในการ์ดเป็นค่า ณ เวลาที่ดึง ถามใหม่เพื่อดูค่าล่าสุด`);for(const o of a.messages)Dt(e,o);e.ui.input.focus()}catch(a){_(e.ui.log,"ai",a instanceof E?a.message:I.unavailable,"err")}finally{e.busy=!1}}}function Dt(e,t){var a,o,i,r;if(t.role==="user"){_(e.ui.log,"me",t.text);return}if(!t.text&&!((a=t.cards)!=null&&a.length))return;const n=_(e.ui.log,"ai",t.text,t.status==="error"?"err":"");if((o=t.cards)!=null&&o.length){const s=c("div","cards");for(const f of t.cards)s.appendChild(te(f,{inline:e.inlineCards}));n.appendChild(s)}if((i=t.actions)!=null&&i.length){const s=c("div","acts");for(const f of t.actions)Re(e,s,f);n.appendChild(s)}for(const s of(r=t.suggestions)!=null?r:[])Be(e,n,s,{})}function Be(e,t,n,a){let o=t.querySelector(":scope > .sugs");o||(o=c("div","sugs"),t.appendChild(o)),o.appendChild(Ze(n.label,()=>{e.busy||(e.ui.input.value=n.ask,M(a))}))}function Re(e,t,n){const a=e.pageActions.find(o=>o.id===n.id);!a||!wt(a)||t.appendChild(Qe(a.label||n.label,()=>{$(!1),Ct(a)||($(!0),z(e.ui.log,`เปิด ${a.label} ไม่ได้ ลองกดเมนูที่หน้าเว็บแทน`),A(e.ui.log))}))}function Ot(e){e.ui.input.disabled=!0,e.ui.send.disabled=!0,e.ui.launcher.style.display="none"}async function Bt(e,t,n){var f,g;e.history.push({role:"user",text:n}),e.busy=!0,e.ui.send.disabled=!0;const a=ee(e.ui.log,"กำลังส่งคำถาม…");let o=null,i=null,r="";const s=()=>{if(o)return;const p=_(e.ui.log,"ai","");o=p.querySelector(".txt"),i=c("div","cards"),p.appendChild(i)};try{await ht({apiBase:e.apiBase,token:t.getToken(),messages:e.history.slice(-St),page:(g=(f=t.getPage)==null?void 0:f.call(t))!=null?g:"",on:{status:p=>a.set(p||"กำลังทำงาน…"),card:p=>{s(),i.appendChild(te(p)),A(e.ui.log)},token:p=>{a.remove(),s(),r+=p,o.textContent=r,A(e.ui.log)}}}),r?e.history.push({role:"assistant",text:r}):_(e.ui.log,"ai",I.empty)}catch(p){e.history.pop();const l=p instanceof E?p.message:I.unavailable;_(e.ui.log,"ai",l,"err")}finally{a.remove(),e.busy=!1,e.ui.send.disabled=!1}}function ze(e){var t,n;return e==="light"||e==="dark"?e:(n=(t=window.matchMedia)==null?void 0:t.call(window,"(prefers-color-scheme: dark)"))!=null&&n.matches?"dark":"light"}function Rt(e){return e.replace(/[A-Z]/g,t=>"-"+t.toLowerCase())}function Le(e){Object.defineProperty(window,"__aiOfficeConsole",{value:Object.freeze({mount:n=>K({apiBase:e,console:n}),unmount:V,open:()=>$(!0),close:()=>$(!1),__logText:()=>{var n;return(n=u==null?void 0:u.ui.log.textContent)!=null?n:""},__send:async n=>{u&&(u.ui.input.value=n,await M({}))}}),configurable:!0})}function zt(){Object.defineProperty(window,"__aiOffice",{value:Object.freeze({open:()=>$(!0),close:()=>$(!1),__hasLauncher:()=>!!u&&u.ui.launcher.style.display!=="none",__isOpen:()=>!!u&&u.ui.panel.dataset.open==="true",__launcherStyle:()=>u?u.ui.launcher.style:{},__config:()=>u?{...u.cfg}:{},__host:()=>{var t;return(t=u==null?void 0:u.host)!=null?t:null},__send:async t=>{u&&(u.ui.input.value=t,await M({}))},__logText:()=>{var t;return(t=u==null?void 0:u.ui.log.textContent)!=null?t:""},__conversationID:()=>{var t;return(t=u==null?void 0:u.conversationID)!=null?t:""},__showRooms:()=>De(),__openRoom:async t=>{if(!u)return;const n=await ue(u);await Oe(u,n,t)}}),configurable:!0})}const U=document.currentScript;if(U){const e={};for(const s in U.dataset)e[s]=U.dataset[s];U.src&&(e.apiBase||(e.apiBase=new URL(U.src,location.href).origin));const t=Te((Ne=e.apiBase)!=null?Ne:"",e.hostApiBase);let n=null,a=0,o=null;const i=()=>{try{if(!n){if(Date.now()<a)return;a=Date.now()+3e4,de(t).then(g=>{n=g,g&&i()});return}const s=t.credential(),f=s?s.slice(0,s.indexOf(" "))+":"+(ce(n.service)||"\0"):"";if(f===o)return;o=f,f===""?V():K({dataset:e})}catch{}},r=()=>{var s;if(e.consoleMode!==void 0){Le((s=e.apiBase)!=null?s:"");return}if(e.previewMount){K({dataset:e});return}i(),setInterval(i,800)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",r):r()}return D.__resetSessions=_t,D.installConsoleGlobal=Le,D.mount=K,D.unmount=V,Object.defineProperty(D,Symbol.toStringTag,{value:"Module"}),D})({});
