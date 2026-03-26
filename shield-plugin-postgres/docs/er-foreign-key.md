# ER Diagram — 交互逻辑文档

## Overview

ER 图支持以下操作，通过点击、拖动、右键菜单和 SVG 图标触发，DDL 通过 `POST /api/query` 执行。read-only 模式下所有写操作均被拦截（前端隐藏图标和菜单项 + 后端 403）。

---

## 一、交互模型

### 鼠标区域

ER 图的画布分为多个交互区域，由 `hitTestColumn` 和 `hitTestTable` 判定：

| 区域 | 左键点击 | 左键拖动 | 右键操作 |
|---|---|---|---|
| 表头 | — | 移动表位置 | 表菜单（重命名/删除表、新增字段） |
| 表头齿轮图标 | 打开表结构编辑器 | — | — |
| 字段行 | — | 创建外键 | 字段菜单（编辑/删除字段、新增字段等） |
| 字段行齿轮图标 | 弹出字段操作菜单 | — | — |
| 空白区域 | 取消选中 | 画布平移 | 画布菜单（新建表） |
| 关系线 | 选中/取消 | — | — |

### SVG 齿轮图标

| 位置 | 显示时机 | 点击行为 |
|---|---|---|
| 表头右侧 | 始终显示（非 read-only） | 打开表结构编辑器 |
| 字段行右侧 | **仅鼠标悬停该行时显示** | 弹出上下文菜单（编辑/删除字段、新增字段） |

字段行齿轮采用 hover-only 设计：通过 `hoveredCol` 状态跟踪当前鼠标所在的字段行，仅对该行渲染齿轮图标。鼠标移到其他行时，上一行的图标消失，当前行出现。这样避免每行都显示图标造成视觉噪音。

read-only 模式下所有齿轮图标均不渲染。

### 快捷键

| 按键 | 条件 | 操作 |
|---|---|---|
| `Delete` / `Backspace` | 已选中关系线 | 弹出删除外键弹窗 |
| `Escape` | ER 图可见 | 关闭 ER 图 |
| `Ctrl + 滚轮` | — | 缩放 |
| 滚轮 | — | 平移 |

---

## 二、外键操作

### 2.1 创建外键（拖动字段）

从表 A 的任意字段（包括主键）拖动到表 B，松开鼠标后弹出确认弹窗。

**判定逻辑：**

```
拖动 A.col → 落在 B 上
│
├─ 落在 B 的具体字段行上（hitTestColumn 命中）
│   │
│   ├─ 该字段是 B 的主键 → 跳过，走下方"表头/空白区域"逻辑
│   │
│   ├─ 该字段类型与 A.col 相同 → 直接用该字段建立外键（不创建新字段）
│   │   例：A.user_id(integer) → B.user_id(integer) ✓
│   │
│   └─ 该字段类型与 A.col 不同 → 创建新字段
│       默认字段名：A_col，用户可修改
│
└─ 落在 B 的表头 / 空白区域（未命中具体字段）
    │
    ├─ B 有同名非主键字段 → 直接用该字段
    │   例：A.email → B.email（B.email 非主键）✓
    │
    └─ B 无同名字段，或同名字段是 B 的主键 → 创建新字段
        默认字段名：A_col，用户可修改
        例：A.id → B（B.id 是 B 的主键）→ 创建 B.A_id
```

**主键规则：**
- **源端（A）**：主键可以作为拖动源。
- **目标端（B）**：B 的主键字段不能作为外键目标。

**确认弹窗：**
- 关系图示：`A.col ← B.fk_col`（含类型标注）
- 字段名输入框（仅创建新字段时可编辑）
- SQL 预览（可折叠），字段名修改时实时更新

**执行的 SQL：**

```sql
-- 步骤 1（仅需要创建新字段时）
ALTER TABLE "schema"."B" ADD COLUMN "fk_col" integer;

-- 步骤 2
ALTER TABLE "schema"."B" ADD CONSTRAINT "fk_B_fk_col_A"
  FOREIGN KEY ("fk_col") REFERENCES "schema"."A"("col");
```

约束命名规则：`fk_{目标表}_{外键字段}_{源表}`

**拖动视觉反馈：**

| 状态 | 表现 |
|---|---|
| 拖动中 | 紫色虚线从源字段延伸到鼠标位置 |
| 悬停在目标表上 | 目标表显示紫色虚线边框 |
| 悬停在匹配字段上（类型相同） | 字段行绿色高亮 |
| 悬停在不匹配字段上 | 字段行蓝色高亮 |

### 2.2 删除外键（选中线条）

1. **点击关系线** — 线条变为红色加粗（14px 宽透明命中区域）
2. **再次点击** 或 **点击空白区域** — 取消选中
3. **按 `Delete` / `Backspace`** — 弹出删除确认弹窗

**弹窗内容：**
- 约束名称
- 关系图示（红色背景）
- SQL 预览

**执行的 SQL：**

```sql
ALTER TABLE "schema"."from_table" DROP CONSTRAINT "constraint_name";
```

---

## 三、表操作

### 3.0 表结构编辑器（综合入口）

**触发：** 点击表头（无拖动） 或 点击表头齿轮图标

**弹窗内容：**
- 表名输入框（可直接修改 → RENAME TABLE）
- 字段列表表格，每行显示：PK 标志、名称输入框、类型下拉、删除按钮
- "+ Add Column" 按钮，新增空行
- 删除已有字段：点击删除按钮，行变为半透明+删除线（标记为待删除），再次点击恢复
- SQL 预览（实时显示所有变更的 DDL 语句）
- Save Changes 按钮，顺序执行所有 SQL

**生成的 SQL（按以下顺序）：**

```sql
-- 1. 重命名表（如果表名变更）
ALTER TABLE "schema"."old" RENAME TO "new";

-- 2. 删除标记为待删除的字段
ALTER TABLE "schema"."table" DROP COLUMN "col" CASCADE;

-- 3. 新增的字段
ALTER TABLE "schema"."table" ADD COLUMN "new_col" text;

-- 4. 修改名称的字段
ALTER TABLE "schema"."table" RENAME COLUMN "old_col" TO "new_col";

-- 5. 修改类型的字段
ALTER TABLE "schema"."table" ALTER COLUMN "col" TYPE new_type USING "col"::new_type;
```

**区分点击 vs 拖动：** `dragMoved` 标志在 mousedown 时设为 false，mousemove 时设为 true。mouseup 时 `dragMoved === false` 视为点击，打开编辑器；否则视为拖动，保存位置。

### 3.1 重命名表

**触发：** 右键表头或字段行 → "Rename Table..."

**弹窗：** 当前名称（只读）+ 新名称输入框 + SQL 预览

**执行的 SQL：**

```sql
ALTER TABLE "schema"."old_name" RENAME TO "new_name";
```

**附加逻辑：** 重命名后自动迁移 `tablePositions` 中的位置数据（`old_name → new_name`），保持布局不变。

### 3.2 删除表

**触发：** 右键表头或字段行 → "Delete Table"

**弹窗：**
- 表名
- 如有外键关系，显示警告列表（从 `erData.relations` 中扫描）
- CASCADE / RESTRICT 选择（有 FK 时默认勾选 CASCADE）
- SQL 预览

**执行的 SQL：**

```sql
DROP TABLE "schema"."table_name" CASCADE;  -- 或 RESTRICT
```

### 3.3 新建表

**触发：** 右键空白区域 → "Create Table..."

**弹窗：**
- 表名输入框
- 字段列表（动态增减），每行：名称 + 类型下拉 + PK 复选框 + 删除按钮
- 默认初始一行：`id serial PK`
- "+ Add Column" 按钮
- SQL 预览

**执行的 SQL：**

```sql
CREATE TABLE "schema"."table_name" (
  "id" serial,
  "name" text,
  PRIMARY KEY ("id")
);
```

**附加逻辑：** 新表会放置在右键点击的位置。

---

## 四、字段操作

### 4.1 新增字段

**触发：** 右键表头或字段行 → "Add Column..."

**弹窗：**
- 字段名输入框
- 类型下拉（常用 PG 类型）
- NOT NULL 复选框
- Default 值输入框（可选）
- SQL 预览

**执行的 SQL：**

```sql
ALTER TABLE "schema"."table" ADD COLUMN "col" text [NOT NULL] [DEFAULT ...];
```

### 4.2 编辑字段

**触发：** 右键字段行 → "Edit Column..." 或 点击字段行铅笔图标

**弹窗：**
- 字段名（可修改 → RENAME COLUMN）
- 类型下拉（可修改 → ALTER COLUMN TYPE）
- SQL 预览，实时显示变更的语句

**执行的 SQL（按需生成，顺序执行）：**

```sql
-- 仅名称变更时
ALTER TABLE "schema"."table" RENAME COLUMN "old" TO "new";

-- 仅类型变更时
ALTER TABLE "schema"."table" ALTER COLUMN "col" TYPE new_type USING "col"::new_type;
```

### 4.3 删除字段

**触发：** 右键字段行 → "Delete Column" 或 点击字段行垃圾桶图标

**弹窗：**
- 字段名 + 类型
- 主键警告（如果是 PK）
- 外键警告（如果有 FK 引用，列出关系列表）
- CASCADE / RESTRICT 选择
- SQL 预览

**执行的 SQL：**

```sql
ALTER TABLE "schema"."table" DROP COLUMN "col" [CASCADE | RESTRICT];
```

---

## 五、Read-Only 模式

| 层级 | 行为 |
|---|---|
| 前端 — SVG 图标 | 不渲染齿轮、铅笔、垃圾桶图标 |
| 前端 — 右键菜单 | 仅显示 "Read-only mode"（禁用状态） |
| 前端 — 表头点击 | 不打开编辑器（仅拖动有效） |
| 前端 — 字段拖动 | 不启动 FK 拖动，退化为表拖动或画布平移 |
| 前端 — Delete 键 | 不响应 |
| 后端 — `POST /api/query` | `isWriteSQL()` 拦截 ALTER/CREATE/DROP 等，返回 403 |

前端通过全局变量 `readOnly`（由 `/api/info` 返回值设置）判断。

---

## 六、后端 API

### GET /api/er?schema=public

返回表结构和外键关系，用于渲染 ER 图。

```json
{
  "code": 200,
  "data": {
    "tables": [
      {
        "name": "users",
        "columns": [
          { "name": "id", "type": "integer", "pk": true },
          { "name": "email", "type": "character varying", "pk": false }
        ]
      }
    ],
    "relations": [
      {
        "constraint": "fk_orders_user_id_users",
        "from_table": "orders",
        "from_column": "user_id",
        "to_table": "users",
        "to_column": "id"
      }
    ]
  }
}
```

### POST /api/query

执行所有 DDL 语句（外键、表、字段操作均通过此接口）。read-only 模式下写操作返回 403。

---

## 七、动态表宽度

表格宽度根据字段名和类型的文本长度自动计算，避免文字重叠。

**常量：**

| 常量 | 值 | 说明 |
|---|---|---|
| `MIN_TABLE_WIDTH` | 180 | 最小宽度 |
| `MAX_TABLE_WIDTH` | 400 | 最大宽度 |
| `CHAR_W` | 7 | 字段名字符估算宽度（px） |

**计算逻辑（`computeTableWidth(t)`）：**

1. 表头宽度 = `表名长度 × 8 + 40`（含齿轮图标空间）
2. 每个字段行宽度 = `字段名长度 × 7 + 30` + `类型长度 × 6.5 + 10` + 24（图标空间）
3. 最终宽度 = `min(MAX, max(MIN, headerW, maxColW))`

**调用时机：**

- `loadER()` — 数据加载后、布局/渲染前
- `erSetLayout()` — 切换布局模式时
- `erClearLayout()` — 重置布局时

**辅助函数：**

- `tw(name)` — 返回指定表的宽度，未计算时回退到 `MIN_TABLE_WIDTH`
- `avgTw()` — 所有表宽度的平均值，用于布局算法中的间距计算

---

## 八、代码位置

| 功能 | 文件 | 函数/区域 |
|---|---|---|
| ER 数据接口 | `handler.go` | `erHandler()` |
| SVG 表渲染（含图标） | `er.js` | `renderTableSvg()` — 齿轮/铅笔/垃圾桶图标 |
| SVG 图标点击 | `er.js` | `canvas.click` 事件 → `data-action` 分发 |
| 图标 hover 样式 | `index.html` | `.er-canvas [data-action]:hover` |
| 右键菜单系统 | `er.js` | `showCtxMenu()` / `hideCtxMenu()` / `contextmenu` 事件 |
| 右键菜单样式 | `index.html` | `.er-ctx` / `.er-ctx-item` / `.er-ctx-sep` |
| 点击 vs 拖动区分 | `er.js` | `dragMoved` 标志（mousedown/mousemove/mouseup） |
| 表结构编辑器 | `er.js` | `showTableStructureDialog()` — 综合编辑入口 |
| 外键判定逻辑 | `er.js` | `mouseup` 事件 |
| 拖动视觉反馈 | `er.js` | `renderDragLine()` |
| 创建外键弹窗 | `er.js` | `showFkConfirmDialog()` |
| 删除外键弹窗 | `er.js` | `showFkDeleteDialog()` |
| 线条选中与高亮 | `er.js` | `click` 事件 + `renderRelations()` |
| 重命名表弹窗 | `er.js` | `showRenameTableDialog()` |
| 删除表弹窗 | `er.js` | `showDeleteTableDialog()` |
| 新建表弹窗 | `er.js` | `showCreateTableDialog()` |
| 新增字段弹窗 | `er.js` | `showAddColumnDialog()` |
| 编辑字段弹窗 | `er.js` | `showEditColumnDialog()` |
| 删除字段弹窗 | `er.js` | `showDeleteColumnDialog()` |
| SQL 执行工具 | `er.js` | `execSql()` / `execStmts()` |
| PG 类型列表 | `er.js` | `PG_TYPES` / `pgTypeSelect()` |
| 标识符转义 | `er.js` | `quoteId()` |
| 动态表宽度计算 | `er.js` | `computeTableWidth()` / `computeAllWidths()` / `tw()` / `avgTw()` |
| 操作完成后刷新 | `er.js` | `loadER()`（保持缩放和位置） |
