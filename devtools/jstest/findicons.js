// findicons.js 找出 index.html 中含两个及以上 emoji 的行（双图标按钮）。
"use strict";
const fs = require("fs");
const text = fs.readFileSync("frontend/dist/index.html", "utf8");
const emojiRe = /(\p{Extended_Pictographic}|\u2190|\u2192|\u2191|\u2193|\u21e7|\u21b3|\u21bb|\u21d3|\u2726|\u2600-\u27bf)/gu;
const lines = text.split("\n");
lines.forEach((line, i) => {
  const m = line.match(emojiRe);
  if (m && m.length >= 2) {
    console.log(`${i + 1}: [${m.join(", ")}] ${line.trim().slice(0, 160)}`);
  }
});
// 同样检查 zh/en 字典值里有两个 emoji 的条目
const js = fs.readFileSync("frontend/dist/app.js", "utf8");
js.split("\n").forEach((line, i) => {
  const m = line.match(/"[^"]*":\s*"([^"]*)"/g);
  if (!m) return;
  for (const kv of m) {
    const em = kv.match(emojiRe);
    if (em && em.length >= 2) console.log(`app.js ${i + 1}: [${em.join(", ")}] ${kv.slice(0, 120)}`);
  }
});
