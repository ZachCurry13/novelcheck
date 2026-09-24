# Setting up the AI that rates your books

NovelCheck sends each book's title, author, and back-cover blurb to an AI service, which answers with a rating (spice level, content flags, and a short summary). You choose which service in **Admin → LLM Analysis Engine → AI provider**. Only book information is sent, never your files or anything about your family.

## Which one should I pick?

- **Cheapest and simplest: OpenAI.** About 20 cents per 1,000 books.
- **Most careful answers: Anthropic Claude.** About $1.50 per 1,000 books.
- **Good middle ground: Google Gemini.** Under $1 per 1,000 books, with a limited free tier.
- **Best for obscure or self-published books: Perplexity.** It searches the web for each book. Around $6 per 1,000 books.
- **Free and fully private: Ollama.** Runs on your own server, with a one-click setup inside NovelCheck. A graphics card makes it much faster.

The cost estimates assume about 900 words-worth of text ("tokens") per book. NovelCheck shows your real spending under **Admin → Spent to date**, and you can cap it with **Max tokens per hour**.

Whichever you pick, start with a small batch (5 books), check the ratings look sensible, then run bigger batches.

## OpenAI

1. Go to **platform.openai.com** and sign in or create an account.
2. Add a payment method and some credit: **Settings → Billing → Add payment details**. $5 lasts a long time for NovelCheck.
3. Open **API keys** (in the left menu or under your profile) and click **Create new secret key**. Name it `NovelCheck`.
4. Copy the key right away (it starts with `sk-`). OpenAI only shows it once.
5. In NovelCheck: **Admin → LLM Analysis Engine**, set **AI provider** to **OpenAI**, paste the key into **API key**, and click **Save settings**.
6. Leave the model as `gpt-4o-mini`. It's fast, cheap, and good at this job.

## Anthropic Claude

1. Go to **console.anthropic.com** and sign in or create an account.
2. Add credit: **Settings → Billing**. $5 is plenty to start.
3. Open **API Keys** and click **Create Key**. Name it `NovelCheck`.
4. Copy the key right away (it starts with `sk-ant-`). Anthropic only shows it once.
5. In NovelCheck: **Admin → LLM Analysis Engine**, set **AI provider** to **Anthropic Claude**, paste the key into **API key**, and click **Save settings**.
6. The preset uses **Claude Haiku 4.5** (`claude-haiku-4-5`) for every book and **Claude Sonnet 5** (`claude-sonnet-5`) only when Haiku can't give a usable answer. You can leave both as they are.

Claude doesn't need the "API base URL" or "JSON response mode" settings, so NovelCheck hides them when Claude is selected.

## Google Gemini

1. Go to **aistudio.google.com** and sign in with a Google account.
2. Click **Get API key**, then **Create API key**. Copy the key.
3. Free tier or paid: Gemini has a free tier with daily limits. On the free tier Google may use what you send to improve its products. That's only book blurbs, but if you'd rather it didn't, turn on billing in Google Cloud for that project.
4. In NovelCheck: **Admin → LLM Analysis Engine**, set **AI provider** to **Google Gemini**, paste the key into **API key**, and click **Save settings**.
5. The preset fills in `gemini-2.5-flash`. Google renames models from time to time, so check AI Studio's model list for the current **Flash** model and its price, and update **Primary (small) model** and the two price boxes if they differ.
6. If you hit "rate limit" errors on the free tier, raise **Delay between scans** to 5 seconds or more, or use smaller batches.

## Perplexity

1. Go to **perplexity.ai**, sign in, and open **Settings → API**.
2. Add a payment method and buy some API credit.
3. Click **Generate API key** and copy it (it starts with `pplx-`).
4. In NovelCheck: **Admin → LLM Analysis Engine**, set **AI provider** to **Perplexity**, paste the key into **API key**, and click **Save settings**.
5. The preset uses `sonar`, which searches the web for each book before answering. That's why it's good with lesser-known titles.
6. Perplexity also charges a small fee per request on top of the per-token price. NovelCheck's cost estimate doesn't include that fee, so check your Perplexity usage page for the true total.

JSON response mode is turned off for Perplexity because it doesn't support it. NovelCheck still reads its answers correctly.

## Ollama (free, on your own server)

Ollama runs an AI model on your own TrueNAS box. Nothing leaves your network and there's no bill. With a graphics card (GPU) it's quick; without one, each book can take a minute or more, and ratings are a little less reliable than the paid services.

1. In TrueNAS, open **Apps → Discover Apps**, search for **Ollama**, and click **Install**. If you've set up GPU passthrough, select your GPU in the install form. Leave the other settings as they are and click **Install**, then wait until it shows **Running**.
2. In NovelCheck: **Admin → LLM Analysis Engine**, set **AI provider** to **Ollama**. An **Ollama easy setup** box appears.
3. Click **1. Find Ollama**. NovelCheck looks for Ollama on your server. If it isn't found, type its address in the box next to the button (your TrueNAS IP and the port shown on the Ollama app, for example `192.168.1.50:11434`, or `:30068` for the TrueNAS app) and click **Find Ollama** again. You don't need to type `http://`; NovelCheck adds it.
4. No models yet? Under **3. Download another model**, pick one and click **Download**. A progress bar shows the download.
   - With a GPU: **Qwen 2.5 7B** (about 4.7 GB) gives the best ratings.
   - Without a GPU: **Llama 3.2 3B** (about 2 GB) is the fastest.
5. Under **2. Tick the models to use and put them in order**, tick the models you want and use **↑ / ↓** to order them. **#1** rates every book; if it fails on a book, **#2** tries, then **#3**, and so on. Click **Use these models in this order**. NovelCheck fills in all the settings for you, including $0 prices. Click **Save settings** to keep any other changes you made in the box.
6. Try a batch of 2 or 3 books first to see how long each one takes.

## Other OpenAI-compatible services

Many services and self-hosted tools (vLLM, LM Studio, OpenRouter, Groq, Together, and others) accept the same requests as OpenAI.

1. Set **AI provider** to **Other (OpenAI-compatible)**.
2. Fill in **API base URL** with the address the service gives for its OpenAI-compatible API. It usually ends in `/v1`. NovelCheck adds `/chat/completions` itself.
3. Fill in **API key** and **Primary (small) model** from the service's documentation.
4. Set the two price boxes from the service's pricing page so the cost estimate is right.
5. If ratings fail with an error mentioning `response_format`, untick **JSON response mode**.

## Google Books API key (recommended)

Before rating a book, NovelCheck looks up its back-cover blurb from Open Library and Google Books. Without a key, Google shares one small daily quota among everyone, so big libraries often hit "daily limit reached". A free key fixes that:

1. Go to **console.cloud.google.com** and sign in with a Google account. Create a project if asked (any name, e.g. `NovelCheck`).
2. Open **APIs & Services → Library**, search for **Books API**, and click **Enable**.
3. Open **APIs & Services → Credentials → Create credentials → API key**, and copy the key.
4. In NovelCheck: **Admin → LLM Analysis Engine → Google Books API key**, paste it, and click **Save settings**.
5. Check it: **System → Check everything** should show Google Books "Working with your API key".

The Books API is free for normal use. No billing account is needed.

## Troubleshooting

- **"rejected the API key" or 401 errors:** the key was copied incompletely, or the account has no credit. Create a new key and paste it again.
- **"model not found" or 404 errors:** the model name is wrong or has been retired. Check the provider's current model list.
- **"rate limit" or 429 errors:** you're sending too fast for your plan. Raise **Delay between scans**, lower **Books per batch**, or add credit or upgrade the plan with the provider.
- **Lots of "Analysis Error" books:** open one to read the error. Switching to a stronger **Fallback (large) model** often fixes books the small model struggles with.
- **Costs higher than expected:** lower **Max tokens per hour**, and remember that Perplexity's per-request fee isn't included in the estimate.
