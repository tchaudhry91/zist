const std = @import("std");
const zist = @import("zist");

const version = "0.1.0";

const Command = enum {
    help,
    version,
    collect,
    search,
};

const ParseError = error{
    InvalidCommand,
    MissingArgument,
};

pub fn main() !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const args = try std.process.argsAlloc(allocator);
    if (args.len < 2) {
        try print_help();
        return;
    }

    const base_command = args[1];

    const cmd = try parse_base_command(base_command);

    switch (cmd) {
        .help => try print_help(),
        .version => try print_version(),
        .collect => try cmd_collect(allocator, args),
        .search => try cmd_search(allocator, args),
    }
}

fn parse_base_command(arg: []const u8) ParseError!Command {
    if (std.mem.eql(u8, arg, "--help") or std.mem.eql(u8, arg, "-h")) {
        return .help;
    } else if (std.mem.eql(u8, arg, "--version") or std.mem.eql(u8, arg, "-v")) {
        return .version;
    } else if (std.mem.eql(u8, arg, "collect")) {
        return .collect;
    } else if (std.mem.eql(u8, arg, "search")) {
        return .search;
    }
    return ParseError.InvalidCommand;
}

fn print_help() !void {
    const help_text =
        \\zist - ZSH history aggregation
        \\
        \\Usage: zist <command> [arguments...]
        \\
        \\Commands:
        \\  collect <file>...    Parse history files and update database
        \\  search               Interactive command search
        \\
        \\Options:
        \\  --help, -h           Show this help
        \\  --version, -v        Show version
        \\
    ;
    try zist.bufferedPrint("{s}", .{help_text});
}

fn print_version() !void {
    try zist.bufferedPrint("{s}\n", .{version});
}

fn cmd_collect(allocator: std.mem.Allocator, args: [][:0]u8) !void {
    _ = allocator;
    if (args.len < 3) {
        try zist.bufferedPrint("Error: missing history files\nUsage: zist collect <file>...\n", .{});
        return error.MissingArgument;
    }

    const history_files = args[2..];
    try zist.bufferedPrint("Collecting from {d} files...\n", .{history_files.len});

    for (history_files, 0..) |file, i| {
        try zist.bufferedPrint("  [{d}] {s}\n", .{ i, file });
    }

    try zist.bufferedPrint("Collection complete!\n", .{});
}

fn cmd_search(allocator: std.mem.Allocator, args: [][:0]u8) !void {
    _ = allocator;
    _ = args;
    try zist.bufferedPrint("Search not implemented yet\n", .{});
}
