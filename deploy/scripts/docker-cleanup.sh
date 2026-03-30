#!/bin/bash

# =====================================================
# Docker Full Cleanup Script
# Полная очистка всех Docker ресурсов
# =====================================================

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_step() {
    echo -e "\n${CYAN}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}ℹ️ $1${NC}"
}

# Подсчет ресурсов перед очисткой
count_resources() {
    CONTAINERS=$(docker ps -aq 2>/dev/null | wc -l)
    IMAGES=$(docker images -q 2>/dev/null | wc -l)
    VOLUMES=$(docker volume ls -q 2>/dev/null | wc -l)
    NETWORKS=$(docker network ls -q 2>/dev/null | wc -l)
    
    echo -e "${CYAN}Текущие ресурсы Docker:${NC}"
    echo "   📦 Контейнеров: $CONTAINERS"
    echo "   🖼️  Образов: $IMAGES"
    echo "   💾 Томов: $VOLUMES"
    echo "   🌐 Сетей: $NETWORKS"
}

# Подтверждение действия
confirm_action() {
    echo ""
    print_warning "⚠️  ВНИМАНИЕ: Это действие УДАЛИТ ВСЕ ресурсы Docker!"
    echo "   - Все контейнеры (работающие и остановленные)"
    echo "   - Все образы"
    echo "   - Все тома (данные будут потеряны!)"
    echo "   - Все нестандартные сети"
    echo ""
    read -p "Вы уверены? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Очистка отменена"
        exit 0
    fi
}

# Остановка всех контейнеров
stop_all_containers() {
    print_step "Остановка всех контейнеров"
    
    RUNNING=$(docker ps -q | wc -l)
    if [ "$RUNNING" -gt 0 ]; then
        docker stop $(docker ps -q) 2>/dev/null
        print_success "Остановлено $RUNNING контейнеров"
    else
        print_info "Нет работающих контейнеров"
    fi
}

# Удаление всех контейнеров
remove_all_containers() {
    print_step "Удаление всех контейнеров"
    
    CONTAINERS=$(docker ps -aq | wc -l)
    if [ "$CONTAINERS" -gt 0 ]; then
        docker rm $(docker ps -aq) 2>/dev/null
        print_success "Удалено $CONTAINERS контейнеров"
    else
        print_info "Нет контейнеров для удаления"
    fi
}

# Удаление всех образов
remove_all_images() {
    print_step "Удаление всех образов"
    
    IMAGES=$(docker images -q | wc -l)
    if [ "$IMAGES" -gt 0 ]; then
        docker rmi -f $(docker images -q) 2>/dev/null
        print_success "Удалено $IMAGES образов"
    else
        print_info "Нет образов для удаления"
    fi
}

# Удаление всех томов
remove_all_volumes() {
    print_step "Удаление всех томов"
    
    VOLUMES=$(docker volume ls -q | wc -l)
    if [ "$VOLUMES" -gt 0 ]; then
        docker volume rm $(docker volume ls -q) 2>/dev/null
        print_success "Удалено $VOLUMES томов"
    else
        print_info "Нет томов для удаления"
    fi
}

# Удаление всех нестандартных сетей
remove_all_networks() {
    print_step "Удаление пользовательских сетей"
    
    # Получаем список всех сетей, исключая стандартные
    NETWORKS=$(docker network ls --filter type=custom -q 2>/dev/null | wc -l)
    
    if [ "$NETWORKS" -gt 0 ]; then
        for network in $(docker network ls --filter type=custom -q 2>/dev/null); do
            docker network rm $network 2>/dev/null
        done
        print_success "Удалены пользовательские сети"
    else
        print_info "Нет пользовательских сетей для удаления"
    fi
}

# Очистка системы
prune_system() {
    print_step "Глобальная очистка Docker системы"
    
    # Очистка всего неиспользуемого
    docker system prune -a -f --volumes
    print_success "Система очищена"
}

# Очистка только PigeonGram ресурсов
cleanup_pigeongram() {
    print_step "Очистка только PigeonGram ресурсов"
    
    # Остановка контейнеров PigeonGram
    docker stop $(docker ps -a | grep pigeongram | awk '{print $1}') 2>/dev/null
    
    # Удаление контейнеров PigeonGram
    docker rm $(docker ps -a | grep pigeongram | awk '{print $1}') 2>/dev/null
    
    # Удаление томов PigeonGram
    docker volume rm $(docker volume ls | grep pigeongram | awk '{print $2}') 2>/dev/null
    
    # Удаление сети PigeonGram
    docker network rm pigeongram_network 2>/dev/null
    
    print_success "PigeonGram ресурсы очищены"
}

# Показать статистику после очистки
show_stats() {
    print_step "Статистика после очистки"
    
    echo -e "${CYAN}Оставшиеся ресурсы Docker:${NC}"
    echo "   📦 Контейнеров: $(docker ps -aq 2>/dev/null | wc -l)"
    echo "   🖼️  Образов: $(docker images -q 2>/dev/null | wc -l)"
    echo "   💾 Томов: $(docker volume ls -q 2>/dev/null | wc -l)"
    echo "   🌐 Сетей: $(docker network ls -q 2>/dev/null | wc -l)"
}

# Функция полной очистки
full_cleanup() {
    print_header "ПОЛНАЯ ОЧИСТКА DOCKER"
    
    count_resources
    confirm_action
    
    stop_all_containers
    remove_all_containers
    remove_all_images
    remove_all_volumes
    remove_all_networks
    prune_system
    
    show_stats
    print_success "Полная очистка Docker завершена!"
}

# Функция мягкой очистки (только неиспользуемое)
soft_cleanup() {
    print_header "МЯГКАЯ ОЧИСТКА DOCKER"
    
    count_resources
    
    print_step "Удаление неиспользуемых ресурсов"
    docker system prune -a -f
    
    show_stats
    print_success "Мягкая очистка завершена!"
}

# Функция очистки только PigeonGram
pigeongram_cleanup() {
    print_header "ОЧИСТКА PIGEONGRAM"
    
    count_resources
    cleanup_pigeongram
    show_stats
    print_success "Очистка PigeonGram завершена!"
}

# Показать справку
show_help() {
    print_header "Docker Cleanup Script"
    echo ""
    echo "  Использование: $0 [ОПЦИЯ]"
    echo ""
    echo "  Опции:"
    echo "    full        - Полная очистка ВСЕГО Docker (все контейнеры, образы, тома)"
    echo "    soft        - Мягкая очистка (только неиспользуемые ресурсы)"
    echo "    pigeongram  - Очистка только ресурсов PigeonGram"
    echo "    stats       - Показать статистику использования Docker"
    echo "    help        - Показать эту справку"
    echo ""
    echo "  Примеры:"
    echo "    $0 full      # Полная очистка"
    echo "    $0 soft      # Мягкая очистка"
    echo "    $0 pigeongram # Очистка только PigeonGram"
    echo ""
}

# Основная логика
main() {
    case "${1:-help}" in
        full)
            full_cleanup
            ;;
        soft)
            soft_cleanup
            ;;
        pigeongram)
            pigeongram_cleanup
            ;;
        stats)
            print_header "СТАТИСТИКА DOCKER"
            count_resources
            echo ""
            print_step "Детальная информация"
            docker system df
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Неизвестная опция: $1"
            show_help
            exit 1
            ;;
    esac
}

# Запуск
main "$@"