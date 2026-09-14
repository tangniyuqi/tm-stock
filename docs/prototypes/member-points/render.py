"""生成可复现的会员与贡献流程静态设计稿；只绘制图片，不调用业务接口。"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent
S = 2
RED, INK, MUTED = '#E93030', '#202530', '#858C98'
BG, LINE = '#F5F6F8', '#E9ECF0'

def font(size, bold=False):
    return ImageFont.truetype('C:/Windows/Fonts/msyhbd.ttc' if bold else 'C:/Windows/Fonts/msyh.ttc', size*S)

def rect(x,y,w,h,fill,r=0,outline=None):
    d.rounded_rectangle((x*S,y*S,(x+w)*S,(y+h)*S),radius=r*S,fill=fill,outline=outline,width=S)

def txt(x,y,value,size=14,color=INK,bold=False):
    d.text((x*S,y*S),value,font=font(size,bold),fill=color)

def lines(x,y,value,width=360,size=14,color=INK,step=25):
    row=''
    for ch in value:
        if ch=='\n' or d.textlength((row+ch),font=font(size))>width*S:
            txt(x,y,row,size,color); y+=step; row='' if ch=='\n' else ch
        else: row+=ch
    if row: txt(x,y,row,size,color); y+=step
    return y

def pill(x,y,value,fill='#FFF0EF',color=RED,w=80):
    rect(x,y,w,26,fill,13); txt(x+10,y+4,value,11,color)

def button(y,value,secondary=False):
    rect(24,y,382,48,'#FFFFFF' if secondary else RED,12,RED if secondary else None)
    width=d.textlength(value,font=font(15,True))/S
    txt((430-width)/2,y+13,value,15,RED if secondary else 'white',True)

def shell(title):
    global im,d
    im=Image.new('RGB',(430*S,932*S),BG); d=ImageDraw.Draw(im)
    rect(0,0,430,102,RED); txt(24,12,'9:41',12,'white',True)
    for i in range(4): rect(343+i*5,23-i*3,3,4+i*3,'white',1)
    rect(377,14,25,12,RED,3,'white'); rect(380,17,17,6,'white',1)
    txt(23,49,'‹',30,'white'); txt(156,56,title,19,'white',True)
    rect(0,900,430,32,'white'); rect(154,917,122,4,INK,2)
    txt(24,879,'设计预览 · 示例内容，非真实资料或收费',10,MUTED)

def save(name):
    path=ROOT/name; im.save(path); return im.copy()

shots=[]
shell('题材动态')
rect(18,119,394,67,'#FFF6E8',14); txt(34,132,'会员权益已生效',15,'#825D26',True)
txt(34,158,'基础题材可查看 · 详细整理按篇解锁',12,'#97794B')
txt(24,210,'关联题材',21,INK,True); txt(335,217,'全部折叠',12,MUTED)
rect(18,251,394,471,'white',18); txt(36,270,'光源',21,INK,True)
d.line([(376*S,288*S),(382*S,282*S),(388*S,288*S)],fill=INK,width=2*S)
pill(36,308,'基础内容',w=76)
lines(36,348,'光源是曝光系统中的一个环节。下方展示公司业务与题材归属的简要资料。',350,14)
rect(34,418,362,37,BG,8); txt(48,428,'公司',12,MUTED); txt(180,428,'归属说明',12,MUTED)
txt(47,475,'公司甲',16,INK,True); txt(47,502,'示例公司',11,MUTED)
txt(180,475,'公开业务资料摘要',13); txt(180,502,'基础依据 ›',12,RED)
rect(35,535,360,1,LINE)
txt(47,554,'公司乙',16,INK,True); txt(47,582,'示例公司',11,MUTED)
txt(180,554,'公开业务资料摘要',13); txt(180,582,'基础依据 ›',12,RED)
rect(34,627,362,68,'#FFF5F4',12); txt(49,640,'深入了解题材资料',14,INK,True)
txt(49,665,'AI 整理 · 社区补充 · 来源核验',11,MUTED); txt(315,649,'查看 ›',14,RED,True)
rect(18,737,394,56,'white',14); txt(36,754,'物镜',16,INK,True); txt(380,752,'›',20,MUTED)
rect(18,805,394,56,'white',14); txt(36,822,'光刻胶',16,INK,True); txt(380,820,'›',20,MUTED)
shots.append(save('01-member-basic.png'))

shell('题材动态')
txt(24,127,'关联题材 / 光源',17,INK,True)
rect(0,165,430,735,'#C9CBCD')
rect(0,193,430,707,'white',24); rect(190,204,50,4,'#DFE2E6',2)
txt(24,233,'公司甲 · 题材资料',20,INK,True); txt(381,232,'×',25,MUTED)
txt(24,285,'AI 题材整理',15,RED,True); txt(167,285,'社区补充',15,MUTED); rect(25,316,87,3,RED,2)
pill(24,339,'内容预览',w=78); txt(115,344,'示例版本 · 尚未解锁',12,MUTED)
txt(24,391,'光源环节业务资料整理',20,INK,True)
lines(24,430,'围绕公开披露文件，整理业务描述、来源摘录及待核验事项。基础归属依据仍可直接查看。',380)
rect(24,525,382,129,BG,14); txt(41,542,'解锁后可查看',15,INK,True)
txt(41,575,'· 完整资料说明与出处',13); txt(41,604,'· 来源对照与资料更新时间',13)
txt(24,677,'本篇价格',13,MUTED); txt(305,677,'待配置积分',14,INK,True)
txt(24,710,'可用积分',13,MUTED); txt(323,710,'待加载',14,MUTED)
txt(24,747,'已购同篇可再次查看，不重复扣分',12,MUTED)
rect(24,782,382,48,'#E4E6EA',12); txt(142,795,'价格确认后解锁',15,MUTED,True)
txt(94,846,'积分不足？ 充值积分  ·  做贡献赚积分',12,RED)
shots.append(save('02-detail-locked.png'))

shell('题材动态')
txt(24,127,'关联题材 / 光源',17,INK,True); rect(0,165,430,735,'#C9CBCD')
rect(0,193,430,707,'white',24); rect(190,204,50,4,'#DFE2E6',2)
txt(24,233,'公司甲 · 题材资料',20,INK,True); txt(381,232,'×',25,MUTED)
txt(24,285,'AI 题材整理',15,MUTED); txt(167,285,'社区补充',15,RED,True); rect(167,316,61,3,RED,2)
pill(24,338,'已解锁',fill='#EBF6F0',color='#287653',w=70)
txt(106,343,'示例作者 · 审核通过状态示意',11,MUTED)
txt(24,384,'从公开资料核对业务归属',20,INK,True)
lines(24,423,'此处展示用户提交并经审核的完整资料。每条业务说明关联原文，区分已披露事实与尚未核实的内容。',378,14)
rect(24,513,382,111,BG,12); txt(40,530,'原文摘录与来源',14,INK,True)
txt(40,560,'示例占位：正式内容须附可核验原文',12,MUTED)
txt(40,589,'查看来源文件 ›',12,RED)
txt(24,650,'资料时点',12,MUTED); txt(126,650,'以原文披露时间为准',12)
txt(24,678,'有疑问？',12,MUTED); txt(126,678,'反馈纠错',12,RED)
rect(24,718,382,43,'#FFF6E8',10); txt(37,731,'本次解锁支持贡献者获得积分奖励',12,'#825D26')
button(782,'补充我的逻辑')
txt(120,846,'分享公开摘要   ·   查看购买记录',12,RED)
shots.append(save('03-detail-unlocked.png'))

shell('补充逻辑')
rect(18,118,394,71,'#FFF6E8',14); txt(34,131,'让有依据的贡献获得回报',15,'#825D26',True)
txt(34,157,'审核通过获积分 · 有效付费阅读获额外奖励',11,'#97794B')
txt(24,211,'关联内容',12,MUTED); txt(24,236,'光源 / 公司甲（示例）',17,INK,True)
txt(24,280,'逻辑标题 *',13,INK,True); rect(24,309,382,47,'white',10); txt(38,324,'用一句话说明你补充的资料',13,MUTED)
txt(24,377,'资料说明 *',13,INK,True); rect(24,406,382,119,'white',12)
lines(38,420,'请描述公开事实及与题材的关系。\n说明依据，不作方向判断。',350,13,MUTED)
txt(24,546,'依据来源 *',13,INK,True); rect(24,575,382,136,'white',12)
txt(38,588,'来源类型',12,MUTED); txt(262,588,'请选择来源  ›',12)
rect(38,619,354,1,LINE); txt(38,634,'原文链接 / 原文摘录 / 披露时点',12,MUTED)
txt(38,677,'＋ 添加来源材料',13,RED)
rect(24,731,15,15,'white',3,outline='#A3A9B2'); txt(48,729,'我确认有权发布，并同意贡献与奖励规则',11,MUTED)
button(782,'提交审核')
txt(76,846,'奖励额度及分成规则待配置 · 审核前不公开',11,MUTED)
shots.append(save('04-contribute.png'))

board=Image.new('RGB',(940*S,2050*S),'#E9ECF1')
bd=ImageDraw.Draw(board)
bd.text((28*S,20*S),'题材宝典 · 会员与积分内容流程',font=font(24,True),fill=INK)
bd.text((28*S,59*S),'方案 A  /  示例内容  /  价格与比例待配置  /  非运行页面',font=font(12),fill=MUTED)
for i,shot in enumerate(shots):
    x=28+(i%2)*454; y=105+(i//2)*969
    board.paste(shot,(x*S,y*S))
board.save(ROOT/'preview.png')
print('已生成四张页面图与一张总览图')
