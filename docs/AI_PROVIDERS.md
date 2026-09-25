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
4. No models yet? Under **3. Download another model**, pick one and click **Download**. A progress bar shows the download. The menu marks each model for your GPU: **⭐ Best**, **💪 Most powerful that fits**, or **⚠️ Too big (slow)**. Click **🎮 Check my GPU** to measure it (it loads your biggest downloaded model for a moment), or pick your GPU's memory size (for example 8 GB) from the menu.
   - With a GPU: **Qwen 2.5 7B** (about 4.7 GB) gives the best ratings.
   - Without a GPU: **Llama 3.2 3B** (about 2 GB) is the fastest.
5. Under **2. Tick the models to use and put them in order**, tick the models you want and use **↑ / ↓** to order them. **#1** rates every book; if it fails on a book, **#2** tries, then **#3**, and so on. Click **Use these models in this order**. NovelCheck fills in all the settings for you, including $0 prices. Click **Save settings** to keep any other changes you made in the box.
6. Try a batch of 2 or 3 books first to see how long each one takes.

## Photos for Check a book

**📷 Check a book** asks your AI to read the title off a photo of the cover. OpenAI (`gpt-4o-mini`), Claude and Gemini models can all read photos. With Ollama you need a vision model such as `llama3.2-vision` or `llava` (plain `llama3.2` can't see images); if none of your models can, NovelCheck says so and you can type the title instead. Android phones read the ISBN barcode themselves, so no AI is needed for that.

## Deep Scan (reading the whole book)

A **🧬 Deep Scan** sends the book's full text to your AI in parts, so it uses far more tokens than a normal rating: a typical novel is around 150,000 tokens, or about 2–5 cents with `gpt-4o-mini`. NovelCheck shows the estimate before every scan. You can pick a separate **Deep Scan model** under **Admin → LLM Analysis Engine**, for example a cheap model with a large context window. With Ollama the book is cut into small parts (about 2,000 words) to fit local models' memory; it's free, but a whole book can take a long time on a small GPU.

## Language

Summaries are written in **English (US)** unless you pick another language under **Admin → LLM Analysis Engine → Language for book summaries**. Small models sometimes answered in the book's own language before; if any existing summaries aren't in English, Admin shows **Re-rate them in English**.

## Backup AI (optional)

A second AI that takes over when the main one fails, for example a second Ollama on another computer, or a cloud AI like OpenAI for when your server is off.

1. **Admin → Backup AI (optional)**: tick **Use a backup AI when the main one fails**.
2. Pick the backup's **AI provider**. For a second Ollama, type its address in the Ollama box (for example `192.168.1.60:11434`), click **Find Ollama**, tick its models in order and click **Use these models in this order**. For a cloud AI, paste its API key and model like in the sections above.
3. Click **Save settings**, then **System → Check everything** to see both AIs answer.

When the main AI fails on a book, NovelCheck tries the backup. If the main server can't be reached at all, it goes straight to the backup instead of waiting on each model. The 🔔 bell tells you when the backup was used. Each AI's cost is counted at its own prices.

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
- **Lots of "Analysis Error" books:** open **Usage** (or **Admin**) and look at **Rating errors**: failed books are grouped by reason with a plain explanation. Fix the cause, then click **Retry**. Adding a stronger model under **Fallback model(s)**, or a **Backup AI**, often fixes books the small model struggles with.
- **Costs higher than expected:** lower **Max tokens per hour**, and remember that Perplexity's per-request fee isn't included in the estimate.
