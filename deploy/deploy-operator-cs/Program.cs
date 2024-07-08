using System.Threading.Tasks;
using Pulumi;
using System.Diagnostics;
using System;
using System.Threading;

class Program
{
    static async Task<int> Main()
    {
        await WaitForDebugger();
        return await Deployment.RunAsync<MyStack>();
    }

    public static async Task<bool> WaitForDebugger(TimeSpan? limit = null)
    {
        limit ??= TimeSpan.FromSeconds(30);
        var source = new CancellationTokenSource(limit.Value);
        
        Console.WriteLine($"◉ Waiting {limit.Value.TotalSeconds} secs for debugger (PID: {Environment.ProcessId})...");

        try
        {
            await Task.Run(async () => {
                while (!Debugger.IsAttached) {
                    await Task.Delay(TimeSpan.FromMilliseconds(100), source.Token);
                }
            }, source.Token);
        }
        catch (OperationCanceledException)
        {
            // it's ok
        }

        Console.WriteLine(Debugger.IsAttached 
            ? "✔ Debugger attached" 
            : "✕ Continuing without debugger");

        return Debugger.IsAttached;
    }
}
