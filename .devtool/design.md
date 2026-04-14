## Go TUI Design System: Bubbletea + Lipgloss

### Stack

- **Bubbletea** — Elm-архитектура для терминала (Model → Update → View)
- **Lipgloss** — стилизация и layout
- **Bubbles** — готовые компоненты (textinput, key bindings)

### Color Palette

```
Accent   #5F9FFF   — заголовки, выделение, теги, активные элементы
Text     #FFFFFF   — основной текст
Muted    #A0AEC0   — подсказки клавиш (help bar)
Faint    #718096   — вторичный текст, описания
Border   #4A5568   — разделители, рамки диалогов
SelectBg #2C5282   — фон выделенной строки

OK       #68D391   — успех, clean-статус
Warn     #F6C950   — предупреждение, dirty-статус
Error    #FC8181   — ошибки
```

### Layout

- Два столбца: левый 40%, правый 60%, разделены `│`
- Адаптивность: подстройка под WindowSizeMsg, минимум 80×24
- Body заполняет всё доступное пространство (h - 4)
- Help bar всегда виден внизу
- Status messages auto-hide через 3 секунды

### Key Bindings

- vim-style: j/k + стрелки
- q / ctrl+c — выход
- tab — переключение фокуса
- : — launcher

### Design Principles

1. Один акцентный цвет #5F9FFF для всего интерактивного
2. Три семантических — ok/warn/err, не смешивать с декором
3. Минимум рамок — только у диалогов (RoundedBorder)
4. Help bar всегда виден
5. Auto-hide статусов — 3 секунды
6. Auto-refresh данных каждые 30 секунд
7. Vim-навигация j/k
