// Account types
export type AuthType = string;
export type ApprovalMode = string;
export type ReplyStatus = string;

export interface APICredentials {
  apiKey: string;
  apiSecret: string;
  accessToken: string;
  accessSecret: string;
  bearerToken: string;
}

export interface Cookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number;
  secure: boolean;
  httpOnly: boolean;
}

export interface BrowserAuth {
  cookies: Cookie[];
  userAgent: string;
  proxyUrl?: string;
}

export interface LLMConfig {
  baseUrl: string;
  apiKey: string;
  model: string;
  temperature: number;
  maxTokens: number;
  persona: string;
}

export interface SearchConfig {
  keywords: string[];
  excludeKeywords: string[];
  blocklist: string[];
  englishOnly: boolean;
  minFaves: number;
  minReplies: number;
  minRetweets: number;
  maxAgeMins: number;
  intervalSecs: number;
}

export interface ReplyConfig {
  approvalMode: ApprovalMode;
  maxReplyLength: number;
  tone: string;
  includeHashtags: boolean;
  signatureText?: string;
}

export interface RateLimits {
  searchesPerHour: number;
  repliesPerHour: number;
  repliesPerDay: number;
  minDelayBetween: number;
}

export interface AccountConfig {
  id: string;
  username: string;
  enabled: boolean;
  authType: AuthType;
  debugMode: boolean;
  apiCredentials?: APICredentials;
  browserAuth?: BrowserAuth;
  llmConfig: LLMConfig;
  searchConfig: SearchConfig;
  replyConfig: ReplyConfig;
  rateLimits: RateLimits;
}

export interface AccountStatus {
  isActive: boolean;
  isRunning: boolean;
  lastActivity: string;
  errorMessage: string;
  repliesSent: number;
  repliesQueued: number;
  rateLimitReset: string;
}

// Tweet types
export interface Tweet {
  id: string;
  authorId: string;
  authorUsername: string;
  authorName: string;
  authorBio: string;
  text: string;
  createdAt: string;
  language: string;
  likeCount: number;
  retweetCount: number;
  replyCount: number;
  viewCount: number;
  conversationId: string;
  inReplyToId?: string;
  threadTweets?: Tweet[];
  matchedKeywords: string[];
  discoveredAt: string;
  accountId: string;
}

export interface Reply {
  id: string;
  tweetId: string;
  accountId: string;
  text: string;
  generatedAt: string;
  postedAt?: string;
  status: ReplyStatus;
  llmTokensUsed: number;
  errorMessage?: string;
  postedReplyId?: string;
}

export interface ApprovalQueueItem {
  reply: Reply;
  originalTweet: Tweet;
  queuedAt: string;
  expiresAt: string;
}

// Metrics types
export interface ProfileSnapshot {
  accountId: string;
  timestamp: string;
  followersCount: number;
  followingCount: number;
  tweetCount: number;
  listedCount: number;
}

export interface ReplyPerformanceReport {
  accountId: string;
  period: string;
  totalReplies: number;
  successfulReplies: number;
  failedReplies: number;
  pendingReplies: number;
  avgLikesPerReply: number;
  avgImpressionsPerReply: number;
  topPerformingReplies: ReplyMetrics[];
}

export interface ReplyMetrics {
  replyId: string;
  accountId: string;
  originalTweetId: string;
  timestamp: string;
  likeCount: number;
  retweetCount: number;
  impressions: number;
}

export interface DailyStats {
  accountId: string;
  date: string;
  tweetsSearched: number;
  repliesGenerated: number;
  repliesSent: number;
  repliesFailed: number;
  tokensUsed: number;
}

// Activity log types
export type ActivityType = string;
export type ActivityLevel = string;

export interface ActivityLog {
  id: string;
  accountId: string;
  type: ActivityType;
  level: ActivityLevel;
  message: string;
  details?: string;
  timestamp: string;
}
