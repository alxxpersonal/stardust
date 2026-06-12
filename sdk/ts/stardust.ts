// Typed TypeScript client for the Stardust HTTP/JSON API (see docs/openapi.yaml).
// Uses the global fetch, so it runs in the browser, Obsidian, Node 18+, and Deno.

export interface Hit {
  path: string;
  title: string;
  heading: string;
  snippet: string;
  score: number;
}

export interface QueryResult {
  query: string;
  mode: string;
  hits: Hit[];
}

export interface Note {
  path: string;
  title: string;
  tags: string[] | null;
  links: string[] | null;
  body: string;
}

export interface Status {
  root: string;
  notes: number;
  chunks: number;
  last_indexed_sha: string;
  embed_model: string;
  vectors: boolean;
  reranker: boolean;
}

export interface BrokenLink {
  from: string;
  target: string;
}

export interface GraphReport {
  notes: number;
  links: number;
  orphans: string[] | null;
  broken: BrokenLink[] | null;
}

export interface BundleItem {
  path: string;
  title: string;
  snippet: string;
  score: number;
}

export interface BundleResult {
  task: string;
  items: BundleItem[];
  markdown: string;
  tokens_estimate: number;
}

export interface DigestResult {
  since: string;
  head: string;
  changed: number;
  markdown: string;
}

export interface IndexStats {
  indexed: number;
  skipped: number;
  deleted: number;
  vectors: boolean;
}

export interface Field {
  name: string;
  type: string;
  required: boolean;
  enum?: string[] | null;
  default?: unknown;
}

export interface CollectionInfo {
  name: string;
  path: string;
  description: string;
  fields: Field[] | null;
  records: number;
}

export interface Predicate {
  field: string;
  op: string;
  value: string;
}

export interface Record {
  path: string;
  title: string;
  frontmatter: Record_ | null;
  body: string;
}

// Record_ is the open frontmatter map (Record is taken by the record type above).
export type Record_ = { [key: string]: unknown };

export interface RecordList {
  collection: string;
  folder: string;
  records: Record[];
}

export class StardustClient {
  private readonly baseURL: string;

  constructor(baseURL: string) {
    this.baseURL = baseURL.replace(/\/$/, "");
  }

  private async request<T>(method: "GET" | "POST", path: string, params?: Record<string, string>): Promise<T> {
    const qs = params && Object.keys(params).length ? "?" + new URLSearchParams(params).toString() : "";
    const res = await fetch(this.baseURL + path + qs, { method });
    if (!res.ok) {
      throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as T;
  }

  private async requestJSON<T>(method: "POST" | "PATCH", path: string, body: unknown, params?: Record<string, string>): Promise<T> {
    const qs = params && Object.keys(params).length ? "?" + new URLSearchParams(params).toString() : "";
    const res = await fetch(this.baseURL + path + qs, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as T;
  }

  query(q: string, limit = 10): Promise<QueryResult> {
    return this.request("GET", "/query", { q, limit: String(limit) });
  }

  note(path: string): Promise<Note> {
    return this.request("GET", "/note", { path });
  }

  status(): Promise<Status> {
    return this.request("GET", "/status");
  }

  graph(): Promise<GraphReport> {
    return this.request("GET", "/graph");
  }

  bundle(task: string, budget = 4000): Promise<BundleResult> {
    return this.request("GET", "/bundle", { task, budget: String(budget) });
  }

  digest(since = "", advance = false): Promise<DigestResult> {
    const p: Record<string, string> = {};
    if (since) p.since = since;
    if (advance) p.advance = "true";
    return this.request("GET", "/digest", p);
  }

  index(since = ""): Promise<IndexStats> {
    return this.request("POST", "/index", since ? { since } : undefined);
  }

  listCollections(): Promise<CollectionInfo[]> {
    return this.request("GET", "/collections");
  }

  collection(name: string): Promise<CollectionInfo> {
    return this.request("GET", "/collection", { name });
  }

  async listRecords(
    collection: string,
    filter: Predicate[] = [],
    sort = "",
    limit = 0,
    offset = 0,
  ): Promise<RecordList> {
    // Build the query string by hand: `where` repeats, which a plain object
    // params map cannot express.
    const qs = new URLSearchParams();
    qs.set("collection", collection);
    for (const p of filter) qs.append("where", `${p.field}:${p.op}:${p.value}`);
    if (sort) qs.set("sort", sort);
    if (limit > 0) qs.set("limit", String(limit));
    if (offset > 0) qs.set("offset", String(offset));
    const res = await fetch(`${this.baseURL}/records?${qs.toString()}`, { method: "GET" });
    if (!res.ok) {
      throw new Error(`GET /records: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as RecordList;
  }

  getRecord(path: string): Promise<Record> {
    return this.request("GET", "/record", { path });
  }

  createRecord(collection: string, fields: Record_, body = ""): Promise<Record> {
    return this.requestJSON("POST", "/records", { collection, fields, body });
  }

  patchRecord(path: string, fields?: Record_, body?: string): Promise<Record> {
    const payload: Record_ = {};
    if (fields !== undefined) payload.fields = fields;
    if (body !== undefined) payload.body = body;
    return this.requestJSON("PATCH", "/record", payload, { path });
  }

  healthz(): Promise<{ status: string }> {
    return this.request("GET", "/healthz");
  }
}
