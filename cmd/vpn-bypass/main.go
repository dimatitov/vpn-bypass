package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dimatitov/vpn-bypass/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printHelp()
		return nil
	}

	command := os.Args[1]
	service, err := app.New()
	if err != nil {
		return err
	}

	switch command {
	case "add":
		if len(os.Args) != 3 {
			return fmt.Errorf("использование: vpn-bypass add <domain|cidr>")
		}
		return service.Add(os.Args[2])

	case "remove":
		if len(os.Args) != 3 {
			return fmt.Errorf("использование: vpn-bypass remove <domain|cidr>")
		}
		return service.Remove(os.Args[2])

	case "list":
		return service.List()

	case "sync":
		return service.Sync(context.Background())

	case "clear":
		return service.Clear(context.Background())

	case "status":
		return service.Status()

	case "doctor":
		return service.Doctor(context.Background())

	case "watch":
		fs := flag.NewFlagSet("watch", flag.ContinueOnError)
		interval := fs.Duration("interval", time.Minute, "интервал обновления маршрутов")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		return service.Watch(context.Background(), *interval)

	case "version":
		fmt.Println("vpn-bypass dev")
		return nil

	case "help", "-h", "--help":
		printHelp()
		return nil

	default:
		return fmt.Errorf("неизвестная команда %q", command)
	}
}

func printHelp() {
	fmt.Println(`vpn-bypass — домены и IP напрямую, в обход full-tunnel VPN

Команды:
  add <domain|cidr>       добавить домен или IP/подсеть
  remove <domain|cidr>    удалить запись
  list                    показать конфигурацию
  sync                    обновить маршруты
  clear                   удалить созданные маршруты
  status                  показать сохранённое состояние
  doctor                  проверить шлюз и маршрутизацию
  watch --interval 60s    постоянно обновлять маршруты
  version                 показать версию`)
}
