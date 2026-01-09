const std = @import("std");

const ParseError = error{
    InvalidConfig,
    NoHomeDir,
};
pub const Config = struct {
    collection: Collection = .{},
    sync: Sync = .{},
    llm: LLM = .{},

    pub const Collection = struct {
        history_files: []const []const u8 = &.{"~/.zsh_history"},
        machine_name: []const u8 = "auto",
    };

    pub const Sync = struct {
        peers: []const []const u8 = &.{},
    };

    pub const LLM = struct {
        endpoint: ?[]const u8 = null,
        api_key: ?[]const u8 = null,
        model: ?[]const u8 = null,
    };
};

pub fn load(allocator: std.mem.Allocator, path: []const u8) ParseError!Config {
    _ = allocator;
    std.fs.openFileAbsolute(path, std.fs.File.OpenFlags);
    return Config{};
}

fn expandHome(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    if (std.mem.startsWith(u8, path, "~")) {
        const home = std.posix.getenv("HOME") orelse return ParseError.NoHomeDir;
        return std.fs.path.join(allocator, &.{ home, path[1..] });
    }
    return try allocator.dupe(u8, path);
}
