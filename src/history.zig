const std = @import("std");

const log = std.log.scoped(.history);

const ParseError = error{
    InvalidFormat,
    InvalidTimestamp,
    NoHomeDir,
};

pub const Command = struct {
    timestamp: u64,
    duration: u32,
    command_str: []const u8,
};

pub const History = struct {
    commands: []Command,
    contents: ?[]const u8 = null,

    pub fn deinit(self: *History, allocator: std.mem.Allocator) void {
        if (self.contents) |c| allocator.free(c);
        allocator.free(self.commands);
    }

    pub fn parse(allocator: std.mem.Allocator, path: []const u8) !History {
        const expanded_path = try expand_home(allocator, path);
        defer allocator.free(expanded_path);
        const f = try std.fs.openFileAbsolute(expanded_path, .{});
        defer f.close();

        const contents = try f.readToEndAlloc(allocator, 10 * 1024 * 1024); // 10MB max
        var history = try parse_from_string(allocator, contents);
        history.contents = contents; // parse() owns the contents
        return history;
    }

    pub fn parse_from_string(allocator: std.mem.Allocator, contents: []const u8) !History {
        var commands = try std.ArrayList(Command).initCapacity(allocator, 0);

        // TODO: Parse contents
        // Format: `: timestamp:duration;command`
        // Multi-line: lines not starting with `: ` are continuations

        _ = contents;

        return History{
            .commands = try commands.toOwnedSlice(allocator),
        };
    }
};

fn expand_home(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    if (std.mem.startsWith(u8, path, "~")) {
        const home = std.posix.getenv("HOME") orelse return ParseError.NoHomeDir;
        return std.fs.path.join(allocator, &.{ home, path[1..] });
    }
    return try allocator.dupe(u8, path);
}

// =============================================================================
// Tests
// =============================================================================

const testing = std.testing;

test "parse sample history" {
    const contents = @embedFile("testdata/sample.zsh_history");
    var history = try History.parse_from_string(testing.allocator, contents);
    defer history.deinit(testing.allocator);

    // TODO: Add assertions once parsing is implemented
    // try testing.expectEqual(@as(usize, 11), history.commands.len);
}

test "parse single command" {
    const contents = ": 1704067200:0;ls -la";
    var history = try History.parse_from_string(testing.allocator, contents);
    defer history.deinit(testing.allocator);

    // TODO: Add assertions
    // try testing.expectEqual(@as(usize, 1), history.commands.len);
    // try testing.expectEqual(@as(u64, 1704067200), history.commands[0].timestamp);
    // try testing.expectEqual(@as(u32, 0), history.commands[0].duration);
    // try testing.expectEqualStrings("ls -la", history.commands[0].command_str);
}

test "parse multi-line command" {
    const contents =
        \\: 1704067300:0;echo "line1
        \\line2
        \\line3"
    ;
    var history = try History.parse_from_string(testing.allocator, contents);
    defer history.deinit(testing.allocator);

    // TODO: Add assertions
    // try testing.expectEqual(@as(usize, 1), history.commands.len);
    // Command should include all three lines
}

test "empty history returns empty slice" {
    var history = try History.parse_from_string(testing.allocator, "");
    defer history.deinit(testing.allocator);

    try testing.expectEqual(@as(usize, 0), history.commands.len);
}
