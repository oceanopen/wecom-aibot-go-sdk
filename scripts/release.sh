#!/usr/bin/env bash
# scripts/release.sh 发版脚本（Go 侧自有，效果类似 Node 生态 bumpp 的 pnpm release）。
#
# 用法：
#   ./scripts/release.sh                # 交互式选择 patch / minor / major / 自定义
#   ./scripts/release.sh patch          # 非交互递增，如 0.1.1 → 0.1.2
#   ./scripts/release.sh minor          # 0.1.1 → 0.2.0
#   ./scripts/release.sh major          # 0.1.1 → 1.0.0
#   ./scripts/release.sh 0.2.0-rc.1     # 显式指定版本（支持 pre-release）
#   ./scripts/release.sh --dry-run ...  # 演练：只打印动作，不修改文件与 git
#
# 行为：校验（工作区干净、version.go 与最新 tag 一致）→ 回写新版本 → go build 验证 →
#       commit "chore(release): vX.Y.Z" → annotated tag vX.Y.Z → push 分支 + tag。
set -euo pipefail

VERSION_FILE="aibot/version.go"
DRY_RUN=0
BUMP=""

usage() {
  cat <<'EOF'
用法：./scripts/release.sh [patch|minor|major|版本号] [--dry-run]

  patch / minor / major   递增对应版本位
  版本号                  显式指定（支持 pre-release，如 0.2.0-rc.1）
  --dry-run               演练模式：只打印动作，不修改文件与 git
  无参数                  交互式选择

行为：校验 → 回写 aibot/version.go → go build 验证 →
      commit "chore(release): vX.Y.Z" → annotated tag vX.Y.Z → push 分支 + tag
EOF
  exit 0
}

die() { echo "❌ $*" >&2; exit 1; }
info() { echo "ℹ️  $*"; }

# run：dry-run 感知的命令执行器（dry-run 时仅打印命令）
run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

# is_valid_semver：校验语义化版本（允许 pre-release 后缀，如 0.2.0-rc.1）
is_valid_semver() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]
}

# next_version：按类型递增版本（$1=当前版本，$2=patch/minor/major）
next_version() {
  local major minor patch
  IFS='.' read -r major minor patch <<<"${1%%-*}"
  case "$2" in
    patch) echo "$major.$minor.$((patch + 1))" ;;
    minor) echo "$major.$((minor + 1)).0" ;;
    major) echo "$((major + 1)).0.0" ;;
  esac
}

# ---------- 参数解析 ----------

for arg in "$@"; do
  case "$arg" in
    -h|--help) usage ;;
    --dry-run) DRY_RUN=1 ;;
    patch|minor|major)
      [[ -n "$BUMP" ]] && die "bump 类型/版本只能指定一次"
      BUMP="$arg"
      ;;
    *)
      is_valid_semver "$arg" || die "无效参数：${arg}（可用：patch/minor/major/版本号/--dry-run）"
      [[ -n "$BUMP" ]] && die "版本与 bump 类型不能同时指定"
      BUMP="$arg"
      ;;
  esac
done

# ---------- 前置校验 ----------

command -v git >/dev/null 2>&1 || die "未安装 git"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "不在 git 仓库中"
cd "$(git rev-parse --show-toplevel)"
[[ -f "$VERSION_FILE" ]] || die "缺少 $VERSION_FILE"

# 工作区干净校验仅针对真实发版（dry-run 是只读演练，允许在脏工作区预览）
if [[ "$DRY_RUN" != "1" ]]; then
  [[ -z "$(git status --porcelain)" ]] || die "工作区不干净，请先提交或暂存改动"
fi

CUR=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$VERSION_FILE")
is_valid_semver "$CUR" || die "$VERSION_FILE 中的 Version 常量格式异常：$CUR"

TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)
if [[ -n "$TAG" && "$TAG" != "v$CUR" ]]; then
  die "版本漂移：version.go=${CUR}，最新 tag=${TAG}，请先对齐再发版"
fi

# ---------- 计算新版本 ----------

if [[ -z "$BUMP" ]]; then
  echo "当前版本：${CUR}（tag ${TAG:-无}）"
  echo "请选择新版本："
  echo "  1) patch  → $(next_version "$CUR" patch)"
  echo "  2) minor  → $(next_version "$CUR" minor)"
  echo "  3) major  → $(next_version "$CUR" major)"
  echo "  4) 自定义（如 0.2.0-rc.1）"
  printf "选择 [1-4]："
  read -r choice
  case "$choice" in
    1) BUMP="patch" ;;
    2) BUMP="minor" ;;
    3) BUMP="major" ;;
    4)
      printf "输入新版本："
      read -r BUMP
      is_valid_semver "$BUMP" || die "非法版本号：$BUMP"
      ;;
    *) die "无效选择：$choice" ;;
  esac
fi

if [[ "$BUMP" == "patch" || "$BUMP" == "minor" || "$BUMP" == "major" ]]; then
  NEW=$(next_version "$CUR" "$BUMP")
else
  NEW="$BUMP"
fi

[[ "$NEW" != "$CUR" ]] || die "新版本与当前版本相同：$NEW"
[[ -z "$(git tag -l "v$NEW")" ]] || die "tag v$NEW 已存在"

echo
info "发版：${CUR} → ${NEW}（tag v${NEW}）"
if [[ "$DRY_RUN" == "1" ]]; then
  info "（dry-run 演练模式，不会实际修改文件与 git）"
fi

# ---------- 回写版本号 ----------

if [[ "$DRY_RUN" == "1" ]]; then
  echo "[dry-run] sed -i 回写 ${VERSION_FILE}：Version \"$CUR\" → \"$NEW\""
else
  # sed -i.bak 写法兼容 BSD（macOS）/ GNU sed，替换后清理备份
  sed -i.bak "s/^const Version = \"[^\"]*\"$/const Version = \"$NEW\"/" "$VERSION_FILE"
  rm -f "$VERSION_FILE.bak"
  grep -q "const Version = \"$NEW\"" "$VERSION_FILE" || die "版本号回写失败"
fi

# ---------- 构建验证 ----------

run go build ./...

# ---------- commit + tag + push ----------

run git add "$VERSION_FILE"
run git commit -m "chore(release): v$NEW"
run git tag -a "v$NEW" -m "release v$NEW"
run git push --follow-tags

echo
echo "✅ 发版完成：v$NEW"
echo "   引用新版本：go get github.com/oceanopen/wecom-aibot-go-sdk/aibot@v$NEW"
