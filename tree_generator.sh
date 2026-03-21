#!/bin/bash
# save as: tree_generator.sh
# make executable: chmod +x tree_generator.sh
# run: ./tree_generator.sh

# Получаем директорию, где находится скрипт
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Переходим в эту директорию
cd "$SCRIPT_DIR" || exit

# Создаем файл с деревом проекта
OUTPUT_FILE="project_tree.txt"

echo "Генерация дерева проекта в: $SCRIPT_DIR"
echo "Создаю файл: $OUTPUT_FILE"

# Проверяем, установлена ли утилита tree
if command -v tree &> /dev/null; then
    # Используем tree с красивым форматированием
    tree -a -I ".git|__pycache__|*.pyc|node_modules|.idea|.vscode" > "$OUTPUT_FILE"
else
    # Если tree не установлен, используем find + sed
    echo "Утилита tree не найдена, использую альтернативный метод..." >&2
    {
        echo "."
        find . -type d -o -type f | \
            grep -v "^\./\.git" | \
            grep -v "__pycache__" | \
            grep -v "node_modules" | \
            sort | \
            sed -e 's/[^-][^\/]*\//  /g' -e 's/^/  /' -e 's/-/|/'
    } > "$OUTPUT_FILE"
fi

echo "Готово! Дерево проекта сохранено в: $OUTPUT_FILE"