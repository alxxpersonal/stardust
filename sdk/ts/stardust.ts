// Typed TypeScript client for the Stardust JSON-RPC API (POST /rpc, see docs/openrpc.json).
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

interface RPCError {
  code: number;
  message: string;
}

interface RPCEnvelope<T> {
  result?: T;
  error?: RPCError;
}

export class StardustClient {
  private readonly rpcURL: string;
  private readonly baseURL: string;
  private id = 0;

  constructor(baseURL: string) {
    this.baseURL = baseURL.replace(/\/$/, "");
    this.rpcURL = this.baseURL + "/rpc";
  }

  // call sends one JSON-RPC 2.0 request to POST /rpc and unwraps the result.
  private async call<T>(method: string, params?: unknown): Promise<T> {
    const body: { jsonrpc: "2.0"; id: number; method: string; params?: unknown } = {
      jsonrpc: "2.0",
      id: ++this.id,
      method,
    };
    if (params !== undefined) body.params = params;
    const res = await fetch(this.rpcURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      throw new Error(`POST /rpc ${method}: ${res.status} ${await res.text()}`);
    }
    const env = (await res.json()) as RPCEnvelope<T>;
    if (env.error) {
      throw new Error(`rpc ${method}: ${env.error.code} ${env.error.message}`);
    }
    return env.result as T;
  }

  query(q: string, limit = 10): Promise<QueryResult> {
    return this.call("query", { query: q, limit });
  }

  note(path: string): Promise<Note> {
    return this.call("note/get", { path });
  }

  status(): Promise<Status> {
    return this.call("status");
  }

  graph(): Promise<GraphReport> {
    return this.call("graph");
  }

  bundle(task: string, budget = 4000): Promise<BundleResult> {
    return this.call("bundle", { task, budget });
  }

  digest(since = "", advance = false): Promise<DigestResult> {
    return this.call("digest", { since, advance });
  }

  index(since = ""): Promise<IndexStats> {
    return this.call("index/run", { since });
  }

  listCollections(): Promise<CollectionInfo[]> {
    return this.call("collection/list");
  }

  collection(name: string): Promise<CollectionInfo> {
    return this.call("collection/get", { name });
  }

  listRecords(
    collection: string,
    filter: Predicate[] = [],
    sort = "",
    limit = 0,
    offset = 0,
  ): Promise<RecordList> {
    return this.call("record/list", { collection, filter, sort, limit, offset });
  }

  getRecord(path: string): Promise<Record> {
    return this.call("record/get", { path });
  }

  createRecord(collection: string, fields: Record_, body = ""): Promise<Record> {
    return this.call("record/create", { collection, fields, body });
  }

  patchRecord(path: string, fields?: Record_, body?: string): Promise<Record> {
    const params: { path: string; fields?: Record_; body?: string } = { path };
    if (fields !== undefined) params.fields = fields;
    if (body !== undefined) params.body = body;
    return this.call("record/patch", params);
  }

  // healthz stays a plain REST liveness probe; GET /healthz is retained alongside POST /rpc.
  async healthz(): Promise<{ status: string }> {
    const res = await fetch(this.baseURL + "/healthz", { method: "GET" });
    if (!res.ok) {
      throw new Error(`GET /healthz: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as { status: string };
  }
}
