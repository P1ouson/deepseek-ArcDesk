package main

type App struct{}

func (a *App) Submit(msg string) error {
	_ = a.doSubmit(msg)
	return nil
}

func (a *App) UnusedBind() error {
	return nil
}

func (a *App) OnlyInGo() error {
	return nil
}

func (a *App) doSubmit(msg string) string {
	return msg
}
