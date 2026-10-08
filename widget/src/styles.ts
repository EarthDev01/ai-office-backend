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
`
