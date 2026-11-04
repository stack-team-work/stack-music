# 修复：列表未铺满下方（v1.2.1）

## 根因
`SongTable` 组件自身的根元素没有高度约束，撑满能力来自调用页面的 `.table { flex:1; min-height:0 }` class：
- **曲库页**传了 `class="table"` → 能撑满；
- **收藏 / 播放历史 / 歌单详情页**没传 class → 表格高度 = 内容高度，下方留出大块空白（你截图看到的问题）。

## 修复方案（组件内兜底，所有页面统一生效）

1. **`SongTable.vue`**：根 `NDataTable` 固定加 `class="song-table"`；
2. **`style.css`**：新增全局
   ```css
   .song-table {
     flex: 1;
     min-height: 0;
     height: 100%;
   }
   ```
   让任意页面里的歌曲表格都自动占满剩余高度；NDataTable 的 `flex-height` 依托该确定高度滚动词曲；
3. **四个视图 + 歌单详情统一布局**：`.page` 均为 `height:100%; min-height:0; display:flex; flex-direction:column; gap:12px`（逐个核对，歌单详情与列表两态都覆盖）；
4. 内容区底部仅保留 12px padding（表格卡片贴底不悬空）。

## 验证
1. `vue-tsc` + `npm build` + `wails build`；
2. CDP 逐个页面测量：曲库 / 收藏 / 历史 / 歌单详情 中 `.song-table` 底边与播放条顶边的间距 ≤ 16px（预期 12px padding）；
3. 回归：播放、搜索、歌单操作不受影响（纯布局改动）。

## 风险
纯 CSS / class 修补，不涉及逻辑；若表格仍不撑满（flex-height 解析问题），备选方案是给表格外层容器显式 `height:100%` 或改为外层滚动，计划实施时按实测结果选择。
