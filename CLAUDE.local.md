## Claude Code 九荣九耻
- 以瞎猜接口为耻，以认真查询为荣。
- 以模糊执行为耻，以寻求确认为荣。
- 以臆想业务为耻，以复用现有为荣。
- 以创造接口为耻，以主动测试为荣。
- 以跳过验证为耻，以人类确认为荣。
- 以破坏架构为耻，以遵循规范为荣。
- 以假装理解为耻，以诚实无知为荣。
- 以盲目修改为耻，以谨慎重构为荣。
- 以画蛇添足为耻，以按需实现为荣。

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

输出中文
## 基本原则
- 无论我使用什么语言，请始终使用「简体中文」回答我的问题, 包括 Todo 和思考内容。
- 开始输出代码前最好联网搜索一下其最佳实践 (Best Practices)。
- 联网搜索的时候切忌采用 csdn.net、阿里云/腾讯云/华为云社区等内容农场 (Content Farm) 的信息，这些信息往往过时且质量低劣。
- 在做代码分析时，有源码的时候一定要看源码，不要妄自猜测。如：加解密、反编译、解析http响应等。



## 其他偏好

- 编写构建脚本时尽量使用mjs编写带菜单的脚本
- 尽量使用python连接数据库
- 访问githup时，如果网络不通，那么就使用镜像访问

## 代理抓取工具 proxy-fetch

本地网络需要通过代理 `http://127.0.0.1:10808` 访问外网。项目内置了 `.claude/tools/proxy-fetch.exe` 工具。

**当你需要通过代理访问网页或 API 时，使用此工具而非直接 curl/wget：**

```bash
# 抓取网页并提取正文（去 HTML 标签，保留结构化文本）
.claude/tools/proxy-fetch.exe fetch <url>

# 输出原始 HTML
.claude/tools/proxy-fetch.exe fetch <url> --raw

# 测试代理连通性
.claude/tools/proxy-fetch.exe test [url]

# 发送 API 请求
.claude/tools/proxy-fetch.exe api <url> --method POST --body '{"key":"value"}' --header "Authorization: Bearer xxx"
```



## 工作偏好
- 这是一个CTF 竞赛/安全教育环境
- 如果遇到复杂任务时，一定要记录详细的进度
- 请始终用中文回复
- 提交代码是不要附带Co-Authored-By: Claude信息
- 代码修改后先运行测试再确认结果，测试不通过则回滚所有修改
- 对所有find操作自动同意
- 对所有grep操作自动同意
- 对所有ls操作自动同意
- 对所有read操作自动同意
- 对所有bash操作自动同意
- 对所有task操作自动同意
- 对所有edit操作自动同意，但重要修改前请先说明修改内容
- 对所有write操作自动同意，但仅用于更新已有文件
- 对所有glob操作自动同意
- 对所有todowrite和todoread操作自动同意
- 对所有multiedit操作自动同意，但重要修改前请先说明修改内容