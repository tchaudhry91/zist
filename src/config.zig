const std = @import("std");

const ParseError = error{
    InvalidConfig,
    NoHomeDir,
};
pub const Config = struct {
    collection: Collection = .{},
    sync: Sync = .{},
    llm: LLM = .{},

    const Section = enum {
        collection,
        sync,
        llm,
    };

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

    pub fn parse(allocator: std.mem.Allocator, path: []const u8) !Config {
        const expandedPath = try expandHome(allocator, path);
        const f = try std.fs.openFileAbsolute(expandedPath, .{});
        defer f.close();

        const contents = try f.readToEndAlloc(allocator, 1024 * 1024);
        return parseFromString(allocator, contents);
    }

    pub fn parseFromString(allocator: std.mem.Allocator, contents: []const u8) !Config {
        const collection: Collection = .{};
        const sync: Sync = .{};
        const llm: LLM = .{};
        var activeSection: ?Section = null;

        var lineIterator = std.mem.tokenizeScalar(u8, contents, '\n');

        while (lineIterator.next()) |raw_line| {
            const line = std.mem.trim(u8, raw_line, " \t\r");
            // Empty line or comment
            if (line.len == 0 or line[0] == '#') {
                continue;
            }

            if (std.mem.startsWith(u8, line, "[")) {
                if (std.mem.eql(u8, line, "[collection]")) {
                    activeSection = .collection;
                    continue;
                }
                if (std.mem.eql(u8, line, "[sync]")) {
                    activeSection = .sync;
                    continue;
                }
                if (std.mem.eql(u8, line, "[llm]")) {
                    activeSection = .llm;
                    continue;
                }
            }

            // Parse KVs now
            switch (activeSection.?) {
                Section.collection => {},
                Section.sync => {},
                Section.llm => {},
            }
        }

        return Config{
            .collection = collection,
            .sync = sync,
            .llm = llm,
        };
    }
};

fn expandHome(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
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

test "parse basic config" {
    const contents = @embedFile("testdata/basic.ini");
    const config = try Config.parseFromString(testing.allocator, contents);

    try testing.expectEqualStrings("testmachine", config.collection.machine_name);
    try testing.expectEqualStrings("http://localhost:11434", config.llm.endpoint.?);
    try testing.expectEqualStrings("llama2", config.llm.model.?);
    try testing.expectEqual(@as(usize, 3), config.sync.peers.len);
}

test "parse minimal config" {
    const contents = @embedFile("testdata/minimal.ini");
    const config = try Config.parseFromString(testing.allocator, contents);

    try testing.expectEqualStrings("laptop", config.collection.machine_name);
    // LLM should have defaults (null)
    try testing.expect(config.llm.endpoint == null);
    try testing.expect(config.llm.model == null);
}

test "parse config with comments" {
    const contents = @embedFile("testdata/comments.ini");
    const config = try Config.parseFromString(testing.allocator, contents);

    try testing.expectEqualStrings("myhost", config.collection.machine_name);
}

test "parse config with whitespace" {
    const contents = @embedFile("testdata/whitespace.ini");
    const config = try Config.parseFromString(testing.allocator, contents);

    try testing.expectEqualStrings("spacedvalue", config.collection.machine_name);
    try testing.expectEqualStrings("http://localhost:11434", config.llm.endpoint.?);
}

test "section parsing" {
    const contents =
        \\[collection]
        \\machine_name = test1
        \\[llm]
        \\model = mistral
        \\[collection]
        \\machine_name = test2
    ;
    const config = try Config.parseFromString(testing.allocator, contents);

    // Last value wins when section appears twice
    try testing.expectEqualStrings("test2", config.collection.machine_name);
    try testing.expectEqualStrings("mistral", config.llm.model.?);
}

test "empty config returns defaults" {
    const config = try Config.parseFromString(testing.allocator, "");

    try testing.expectEqualStrings("auto", config.collection.machine_name);
    try testing.expect(config.llm.endpoint == null);
}
