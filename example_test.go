package e2b_test

import (
	"context"
	"fmt"
	"log"

	"github.com/xerpa-ai/e2b-go"
)

func Example() {
	// Create a new sandbox
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	// Execute Python code
	execution, err := sandbox.RunCode(context.Background(), "x = 1 + 1; x")
	if err != nil {
		log.Fatal(err)
	}

	// Access the result
	fmt.Println(execution.Text())
}

func Example_streaming() {
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	// Execute code with streaming callbacks
	code := `
for i in range(3):
    print(f"Count: {i}")
`

	execution, err := sandbox.RunCode(
		context.Background(),
		code,
		e2b.OnStdout(func(msg e2b.OutputMessage) {
			fmt.Printf("stdout: %s\n", msg.Line)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Access final logs
	fmt.Printf("Total stdout lines: %d\n", len(execution.Logs.Stdout))
}

func Example_multiLanguage() {
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	// Execute JavaScript code
	execution, err := sandbox.RunCode(
		context.Background(),
		"const x = [1, 2, 3].map(n => n * 2); x",
		e2b.WithLanguage(e2b.LanguageJavaScript),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(execution.Text())
}

func Example_contexts() {
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	ctx := context.Background()

	// Create an isolated context
	execCtx, err := sandbox.CreateContext(ctx,
		e2b.WithContextLanguage(e2b.LanguagePython),
		e2b.WithCWD("/home/user/project"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Execute code in the context
	_, err = sandbox.RunCode(ctx, "x = 42", e2b.WithContext(execCtx))
	if err != nil {
		log.Fatal(err)
	}

	// Variable persists in the same context
	execution, err := sandbox.RunCode(ctx, "x * 2", e2b.WithContext(execCtx))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(execution.Text())

	// Clean up the context
	if err := sandbox.RemoveContext(ctx, execCtx.ID); err != nil {
		log.Fatal(err)
	}
}

func Example_errorHandling() {
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	// Execute code that raises an error
	execution, err := sandbox.RunCode(context.Background(), "1 / 0")
	if err != nil {
		log.Fatal(err)
	}

	// Check for execution error
	if execution.Error != nil {
		fmt.Printf("Error: %s\n", execution.Error.Name)
		fmt.Printf("Message: %s\n", execution.Error.Value)
	}
}

func Example_envVars() {
	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Close()

	// Execute code with environment variables
	execution, err := sandbox.RunCode(
		context.Background(),
		"import os; os.environ.get('MY_VAR')",
		e2b.WithRunEnvVars(map[string]string{
			"MY_VAR": "hello",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(execution.Text())
}
