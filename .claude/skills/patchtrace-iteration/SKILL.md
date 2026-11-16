---
name: patchtrace-iteration
description: Use when starting or resuming the PatchTrace iteration; routes to requirement.md, task.md, todo.md and enforces evidence-based progress.
---

# PatchTrace 迭代执行

任何新会话开始或恢复 PatchTrace 迭代时，先走本流程，再动代码。
三份文档是唯一事实源；本页只给路由与纪律，完整步骤见 `references/workflow.md`。

## 读取顺序（固定，不得跳读）

1. **`docs/plans/requirement.md`** — 确认需求编号 `R*`、全局约束与红线（§6、§7）。
2. **`docs/plans/task.md`** — 确认所属 Task 的 Files / Interfaces / Steps；只读该 Task，不改其他 Task 的文件。
3. **`docs/plans/todo.md`** — 确认当前状态与「下一步动作」；只做那一件。

不要先看代码。若三份文档与代码冲突，以文档为准并记为阻塞，不改验收标准。

## 每轮只做一件事

```text
读取三文档 → 执行 todo.md「下一步动作」 → 写失败测试 → 跑失败
→ 最小实现 → 跑通过 → 回写 todo.md（命令 + 真实输出）→ 提交 → 给出下一轮唯一动作
```

- TDD：先写失败测试，再写最小实现；不允许先写实现再补测试。
- 每轮结束必须留下三行：`已完成 / 证据 / 下一轮唯一动作`。
- 完成后回写 `todo.md`：勾选对应 checkbox，附命令与真实输出、变更文件、下一步。
- 连续两轮同一 Task 无进展 → 在 `todo.md` §5 记阻塞，换实现路径，不重复同一招。

## 证据纪律（硬性）

- 没有命令输出就不打勾；不打勾的项写清缺什么。
- 证据必须可复现：命令原样 + 关键输出一行（不要“应该通过”）。
- 不得修改验收标准来通过；结论只允许「通过 / 不通过」，不通过附最小复现。
- 越界检查：新增依赖、动 `factories[:7]` / `factories[:12]`、import `Web/` 都算不通过。

## 阻塞上报（三行）

```text
- 现象：哪条命令、什么输出。
- 原因假设：最可能的一条，不列清单。
- 需要谁决策：问题一句话 + 两个选项。
```

同时写入 `todo.md` §5 阻塞表；不要静默硬扛或改需求。

## 语言与范围

- UI 文案与 prompt 用中文；代码标识（包名、类型、函数、字段）用英文。
- 只改当前 Task 声明的文件；新增能力走计划中的 Task，不顺手重构。
- 判定纪律：拒绝文本不算成功；网络错误/证据不足记 `error` / `inconclusive`，不报 clean。

## References

- `references/workflow.md` — 三文档读取细节、TDD 步骤、todo 回写模板、验收证据、阻塞格式
