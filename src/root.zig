//! By convention, root.zig is the root source file when making a library.
const std = @import("std");

pub fn bufferedPrint(comptime fmt: []const u8, args: anytype) !void {
    var stdout_buffer: [1024]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
    const stdout = &stdout_writer.interface;

    try stdout.print(fmt, args);

    try stdout.flush();
}
