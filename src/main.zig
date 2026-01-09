const std = @import("std");
const zist = @import("zist");

const version = "0.1.0";

const Command = enum {
    help,
    version,
};

const ParseError = error{
    InvalidCommand,
};

pub fn main() !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const args = try std.process.argsAlloc(allocator);
    var base_command: []const u8 = "--help";
    if (args.len > 1) {
        base_command = args[1];
    }

    // Parse Base Command
    const cmd = parse_base_command(base_command) catch .help;

    switch (cmd) {
        .help => try print_help(),
        .version => try print_version(),
    }
    const cfg = try zist.config.Config.parse(allocator, "~/.config/zist/config.ini");
    _ = cfg;
}

fn parse_base_command(arg: []const u8) ParseError!Command {
    if (std.mem.eql(u8, arg, "--help")) {
        return .help;
    } else if (std.mem.eql(u8, arg, "--version")) {
        return .version;
    }
    return ParseError.InvalidCommand;
}

fn print_help() !void {
    const help_text =
        \\zist - P2P ZSH history sync
        \\
        \\Usage: zist <command> [options]
        \\
        \\Commands:
        \\  collect      Capture commands from ZSH history
        \\  search       Interactive command search
        \\  ask          Conversational search using LLM
        \\  sync         Sync with peer machines
        \\  serve-sync   Handle incoming sync request (via SSH)
        \\  install      Set up ZSH integration
        \\  uninstall    Remove ZSH integration
        \\
        \\Options:
        \\  --help       Show this help
        \\  --version    Show version
        \\
    ;
    try zist.bufferedPrint("{s}", .{help_text});
}

fn print_version() !void {
    try zist.bufferedPrint("{s}\n", .{version});
}
