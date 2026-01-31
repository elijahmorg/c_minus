#include "tree_sitter/parser.h"
#include "tree_sitter/alloc.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

enum TokenType {
  MODULE_IDENTIFIER,
  IMPORT_PATH,
  CIMPORT_PATH,
};

enum {
  MAX_PREFIXES = 24,
  MAX_PREFIX_LENGTH = 32,
};

typedef struct {
  uint8_t prefix_count;
  char prefixes[MAX_PREFIXES][MAX_PREFIX_LENGTH];
} Scanner;

static bool is_ident_start(int32_t c) {
  return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '$';
}

static bool is_ident_continue(int32_t c) {
  return is_ident_start(c) || (c >= '0' && c <= '9');
}

static void skip_whitespace(TSLexer *lexer) {
  while (lexer->lookahead == ' ' || lexer->lookahead == '\t' || lexer->lookahead == '\n' || lexer->lookahead == '\r') {
    lexer->advance(lexer, true);
  }
}

static bool scan_identifier(TSLexer *lexer, char *buffer, size_t capacity, size_t *length) {
  if (!is_ident_start(lexer->lookahead)) {
    return false;
  }

  size_t size = 0;
  while (is_ident_continue(lexer->lookahead)) {
    if (size + 1 < capacity) {
      buffer[size++] = (char)lexer->lookahead;
    }
    lexer->advance(lexer, false);
  }

  buffer[size] = '\0';
  *length = size;
  lexer->mark_end(lexer);
  return true;
}

static bool scan_string_literal(TSLexer *lexer, char *buffer, size_t capacity, size_t *length) {
  if (lexer->lookahead == 'L' || lexer->lookahead == 'u' || lexer->lookahead == 'U') {
    lexer->advance(lexer, false);
    if (lexer->lookahead == '8') {
      lexer->advance(lexer, false);
    }
  }

  if (lexer->lookahead != '"') {
    return false;
  }

  lexer->advance(lexer, false);

  size_t size = 0;
  while (!lexer->eof(lexer) && lexer->lookahead != '"' && lexer->lookahead != '\n') {
    if (lexer->lookahead == '\\') {
      lexer->advance(lexer, false);
      if (lexer->eof(lexer)) {
        return false;
      }
    }

    if (size + 1 < capacity) {
      buffer[size++] = (char)lexer->lookahead;
    }
    lexer->advance(lexer, false);
  }

  if (lexer->lookahead != '"') {
    return false;
  }

  lexer->advance(lexer, false);
  buffer[size] = '\0';
  *length = size;
  lexer->mark_end(lexer);
  return true;
}

static bool prefix_exists(const Scanner *scanner, const char *prefix, size_t length) {
  for (uint8_t i = 0; i < scanner->prefix_count; i++) {
    if (strlen(scanner->prefixes[i]) == length && memcmp(scanner->prefixes[i], prefix, length) == 0) {
      return true;
    }
  }

  return false;
}

static void add_prefix(Scanner *scanner, const char *prefix, size_t length) {
  if (length == 0 || length >= MAX_PREFIX_LENGTH || scanner->prefix_count >= MAX_PREFIXES) {
    return;
  }

  if (prefix_exists(scanner, prefix, length)) {
    return;
  }

  memcpy(scanner->prefixes[scanner->prefix_count], prefix, length);
  scanner->prefixes[scanner->prefix_count][length] = '\0';
  scanner->prefix_count++;
}

static bool has_dot(const char *segment, size_t length) {
  for (size_t i = 0; i < length; i++) {
    if (segment[i] == '.') {
      return true;
    }
  }
  return false;
}

static void record_import_prefix(Scanner *scanner, const char *path, size_t length, bool allow_dot_h) {
  const char *segment = path;
  size_t segment_length = length;

  for (size_t i = 0; i < length; i++) {
    if (path[i] == '/') {
      segment = path + i + 1;
      segment_length = length - i - 1;
    }
  }

  if (segment_length == 0) {
    return;
  }

  if (allow_dot_h) {
    if (segment_length > 2 && segment[segment_length - 2] == '.' && segment[segment_length - 1] == 'h') {
      size_t base_length = segment_length - 2;
      if (base_length == 0 || has_dot(segment, base_length)) {
        return;
      }
      add_prefix(scanner, segment, base_length);
      return;
    }
  }

  if (has_dot(segment, segment_length)) {
    return;
  }

  add_prefix(scanner, segment, segment_length);
}

void *tree_sitter_c_minus_external_scanner_create(void) {
  return ts_calloc(1, sizeof(Scanner));
}

void tree_sitter_c_minus_external_scanner_destroy(void *payload) {
  ts_free(payload);
}

unsigned tree_sitter_c_minus_external_scanner_serialize(void *payload, char *buffer) {
  Scanner *scanner = (Scanner *)payload;
  unsigned size = 0;
  uint8_t stored = 0;

  buffer[size++] = 0;

  for (uint8_t i = 0; i < scanner->prefix_count; i++) {
    uint8_t length = (uint8_t)strlen(scanner->prefixes[i]);
    if (size + 1 + length > TREE_SITTER_SERIALIZATION_BUFFER_SIZE) {
      break;
    }

    buffer[size++] = length;
    memcpy(buffer + size, scanner->prefixes[i], length);
    size += length;
    stored++;
  }

  buffer[0] = stored;
  return size;
}

void tree_sitter_c_minus_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  Scanner *scanner = (Scanner *)payload;
  scanner->prefix_count = 0;
  memset(scanner->prefixes, 0, sizeof(scanner->prefixes));

  if (length == 0) {
    return;
  }

  unsigned offset = 0;
  uint8_t count = (uint8_t)buffer[offset++];
  for (uint8_t i = 0; i < count && offset < length; i++) {
    uint8_t prefix_length = (uint8_t)buffer[offset++];
    if (prefix_length == 0 || prefix_length >= MAX_PREFIX_LENGTH) {
      offset += prefix_length;
      continue;
    }
    if (offset + prefix_length > length) {
      break;
    }
    memcpy(scanner->prefixes[scanner->prefix_count], buffer + offset, prefix_length);
    scanner->prefixes[scanner->prefix_count][prefix_length] = '\0';
    scanner->prefix_count++;
    offset += prefix_length;
  }
}

bool tree_sitter_c_minus_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  Scanner *scanner = (Scanner *)payload;

  if (valid_symbols[MODULE_IDENTIFIER]) {
    char identifier[MAX_PREFIX_LENGTH];
    size_t length = 0;
    skip_whitespace(lexer);
    if (!scan_identifier(lexer, identifier, sizeof(identifier), &length)) {
      return false;
    }
    if (!prefix_exists(scanner, identifier, length)) {
      return false;
    }
    lexer->result_symbol = MODULE_IDENTIFIER;
    return true;
  }

  if (valid_symbols[CIMPORT_PATH]) {
    char path[256];
    size_t length = 0;
    skip_whitespace(lexer);
    if (!scan_string_literal(lexer, path, sizeof(path), &length)) {
      return false;
    }
    record_import_prefix(scanner, path, length, true);
    lexer->result_symbol = CIMPORT_PATH;
    return true;
  }

  if (valid_symbols[IMPORT_PATH]) {
    char path[256];
    size_t length = 0;
    skip_whitespace(lexer);
    if (!scan_string_literal(lexer, path, sizeof(path), &length)) {
      return false;
    }
    record_import_prefix(scanner, path, length, false);
    lexer->result_symbol = IMPORT_PATH;
    return true;
  }

  return false;
}
