#!/usr/bin/env python3
# FileSync 图标：圆角蓝底 + 白色双向同步箭头环绕文件
from PIL import Image, ImageDraw
import os

S = 1024
img = Image.new('RGBA', (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)

# ---- 背景：垂直渐变蓝圆角方块 ----
r = 200
c0, c1 = (37, 99, 235), (30, 64, 175)  # #2563eb -> #1e40af
grad = Image.new('RGBA', (S, S))
gd = ImageDraw.Draw(grad)
for y in range(S):
    t = y / S
    col = tuple(int(c0[i] + (c1[i] - c0[i]) * t) for i in range(3)) + (255,)
    gd.line([(0, y), (S, y)], fill=col)

mask = Image.new('L', (S, S), 0)
md = ImageDraw.Draw(mask)
md.rounded_rectangle([0, 0, S, S], radius=r, fill=255)
img.paste(grad, (0, 0), mask)

# ---- 中央白色文件形 ----
fx0, fy0, fx1, fy1 = 330, 300, 694, 724
fold = 110  # 折角
file_mask = Image.new('L', (S, S), 0)
fd = ImageDraw.Draw(file_mask)
fd.rounded_rectangle([fx0, fy0, fx1, fy1], radius=36, fill=255)
# 切掉右上折角
fd.polygon([(fx1, fy0), (fx1, fy0 + fold), (fx1 - fold, fy0)], fill=0)
# 白色重画折角三角
fd.polygon([(fx1 - fold, fy0), (fx1, fy0 + fold), (fx1 - fold, fy0 + fold)], fill=255)
white = Image.new('RGBA', (S, S), (255, 255, 255, 255))
img.paste(white, (0, 0), file_mask)

# 文件内部三条浅蓝横线
d2 = ImageDraw.Draw(img)
for i, w in enumerate([(392, 632), (392, 596), (392, 524)]):
    d2.rounded_rectangle([w[0], 430 + i * 68, w[1], 452 + i * 68], radius=11, fill=(37, 99, 235, 255))

# ---- 环绕双向箭头（左上→右下 右上→左下 暗示同步） ----
def arrow(d, p0, p1, w, head, color):
    import math
    ang = math.atan2(p1[1] - p0[1], p1[0] - p0[0])
    ah = math.radians(150)
    d.line([p0, p1], fill=color, width=w)
    for sign in (1, -1):
        hx = p1[0] + head * math.cos(ang + sign * ah)
        hy = p1[1] + head * math.sin(ang + sign * ah)
        d.line([p1, (hx, hy)], fill=color, width=w)

ax_c = (37, 99, 235, 255)
# 左上角箭头 → 右
arrow(d2, (150, 236), (330, 236), 26, 60, ax_c)
# 右下角箭头 ← 
arrow(d2, (874, 788), (694, 788), 26, 60, ax_c)

img.save('/home/z/my-project/filesync/build/appicon.png')

# ico（多尺寸）
ico_sizes = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
img.save('/home/z/my-project/filesync/build/windows/icon.ico', sizes=ico_sizes)
print('icon ok', os.path.getsize('/home/z/my-project/filesync/build/windows/icon.ico'))
