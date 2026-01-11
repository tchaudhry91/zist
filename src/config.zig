const std = @import("std");

const log = std.log.scoped(.config);

const ParseError = error{
    MissingEquals,
    UnknownSection,
    KeyBeforeSection,
    NoHomeDir,
};
pub const Config = struct {
    collection: Collection = .{},
    sync: Sync = .{},
    llm: LLM = .{},

    contents: ?[]const u8 = null,

    const Section = enum {
        collection,
        sync,
        llm,
    };

    pub const Collection = struct {
        history_files: ?[]const []const u8 = null,
        machine_name: []const u8 = "auto",
    };

    pub const Sync = struct {
        peers: ?[]const []const u8 = null,
    };

    pub const LLM = struct {
        endpoint: ?[]const u8 = null,
        api_key: ?[]const u8 = null,
        model: ?[]const u8 = null,
    };

    pub fn deinit(self: *Config, allocator: std.mem.Allocator) void {
        if (self.contents) |c| allocator.free(c);
        if (self.collection.history_files) |hf| allocator.free(hf);
        if (self.sync.peers) |p| allocator.free(p);
    }

    pub fn parse(allocator: std.mem.Allocator, path: []const u8) !Config {
        const expanded_path = try expand_home(allocator, path);
        defer allocator.free(expanded_path);
        const f = try std.fs.openFileAbsolute(expanded_path, .{});
        defer f.close();

        const contents = try f.readToEndAlloc(allocator, 1024 * 1024);
        var config = try parse_from_string(allocator, contents);
        config.contents = contents; // parse() owns the contents
        return config;
    }

    pub fn parse_from_string(allocator: std.mem.Allocator, contents: []const u8) !Config {
        var collection: Collection = .{};
        var sync: Sync = .{};
        var llm: LLM = .{};
        var active_section: ?Section = null;
        var line_num: usize = 0;

        var line_iterator = std.mem.tokenizeScalar(u8, contents, '\n');

        while (line_iterator.next()) |raw_line| {
            line_num += 1;
            const line = std.mem.trim(u8, raw_line, " \t\r");

            // Empty line or comment
            if (line.len == 0 or line[0] == '#') {
                continue;
            }

            // Section header
            if (std.mem.startsWith(u8, line, "[")) {
                if (std.mem.eql(u8, line, "[collection]")) {
                    active_section = .collection;
                    continue;
                }
                if (std.mem.eql(u8, line, "[sync]")) {
                    active_section = .sync;
                    continue;
                }
                if (std.mem.eql(u8, line, "[llm]")) {
                    active_section = .llm;
                    continue;
                }
                // Unknown section
                log.err("line {d}: unknown section '{s}'", .{ line_num, line });
                return ParseError.UnknownSection;
            }

            // Parse KVs now
            const kv_index = std.mem.indexOf(u8, line, "=") orelse {
                log.err("line {d}: missing '=' in '{s}'", .{ line_num, line });
                return ParseError.MissingEquals;
            };
            const key = std.mem.trim(u8, line[0..kv_index], " \t");
            const value = std.mem.trim(u8, line[kv_index + 1 ..], " \t");

            if (active_section == null) {
                log.err("line {d}: key '{s}' before any section", .{ line_num, key });
                return ParseError.KeyBeforeSection;
            }

            switch (active_section.?) {
                Section.collection => {
                    if (std.mem.eql(u8, key, "history_files")) {
                        var hfs = try std.ArrayList([]const u8).initCapacity(allocator, 1);
                        var hf_iterator = std.mem.tokenizeScalar(u8, value, ',');
                        while (hf_iterator.next()) |hf| {
                            try hfs.append(allocator, hf);
                        }
                        collection.history_files = try hfs.toOwnedSlice(allocator);
                        continue;
                    }
                    if (std.mem.eql(u8, key, "machine_name")) {
                        collection.machine_name = value;
                        continue;
                    }
                },
                Section.sync => {
                    if (std.mem.eql(u8, key, "peers")) {
                        var peers = try std.ArrayList([]const u8).initCapacity(allocator, 0);
                        var peers_iterator = std.mem.tokenizeScalar(u8, value, ',');
                        while (peers_iterator.next()) |peer| {
                            try peers.append(allocator, peer);
                        }
                        sync.peers = try peers.toOwnedSlice(allocator);
                        continue;
                    }
                },
                Section.llm => {
                    if (std.mem.eql(u8, key, "endpoint")) {
                        llm.endpoint = value;
                        continue;
                    }
                    if (std.mem.eql(u8, key, "api_key")) {
                        llm.api_key = value;
                        continue;
                    }
                    if (std.mem.eql(u8, key, "model")) {
                        llm.model = value;
                        continue;
                    }
                },
            }
        }

        return Config{
            .collection = collection,
            .sync = sync,
            .llm = llm,
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

test "parse basic config" {
    const contents = @embedFile("testdata/basic.ini");
    var config = try Config.parse_from_string(testing.allocator, contents);
    defer config.deinit(testing.allocator);

    try testing.expectEqualStrings("testmachine", config.collection.machine_name);
    try testing.expectEqualStrings("http://localhost:11434", config.llm.endpoint.?);
    try testing.expectEqualStrings("llama2", config.llm.model.?);
    try testing.expectEqual(@as(usize, 3), config.sync.peers.?.len);
}

test "parse minimal config" {
    const contents = @embedFile("testdata/minimal.ini");
    var config = try Config.parse_from_string(testing.allocator, contents);
    defer config.deinit(testing.allocator);

    try testing.expectEqualStrings("laptop", config.collection.machine_name);
    // LLM should have defaults (null)
    try testing.expect(config.llm.endpoint == null);
    try testing.expect(config.llm.model == null);
}

test "parse config with comments" {
    const contents = @embedFile("testdata/comments.ini");
    var config = try Config.parse_from_string(testing.allocator, contents);
    defer config.deinit(testing.allocator);

    try testing.expectEqualStrings("myhost", config.collection.machine_name);
}

test "parse config with whitespace" {
    const contents = @embedFile("testdata/whitespace.ini");
    var config = try Config.parse_from_string(testing.allocator, contents);
    defer config.deinit(testing.allocator);

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
    var config = try Config.parse_from_string(testing.allocator, contents);
    defer config.deinit(testing.allocator);

    // Last value wins when section appears twice
    try testing.expectEqualStrings("test2", config.collection.machine_name);
    try testing.expectEqualStrings("mistral", config.llm.model.?);
}

test "empty config returns defaults" {
    var config = try Config.parse_from_string(testing.allocator, "");
    defer config.deinit(testing.allocator);

    try testing.expectEqualStrings("auto", config.collection.machine_name);
    try testing.expect(config.llm.endpoint == null);
}

