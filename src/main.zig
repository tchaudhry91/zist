const std = @import("std");
const zist = @import("zist");

const VERSION = "0.1.0";

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
        .help => try printHelp(),
        .version => try printVersion(),
    }
}

fn parse_base_command(arg: []const u8) ParseError!Command {
    if (std.mem.eql(u8, arg, "--help")) {
        return .help;
    } else if (std.mem.eql(u8, arg, "--version")) {
        return .version;
    }
    return ParseError.InvalidCommand;
}

fn printHelp() !void {
    try zist.bufferedPrint("Usage: Please use the following commands", .{});
}

fn printVersion() !void {
    try zist.bufferedPrint("{s}", .{VERSION});
}
