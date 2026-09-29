# Discover: getting a free New York Times key

The **🧭 Discover** tab shows popular books, new books on the best-seller lists, top teen and kids' books, all-time classics, and your family's new and popular books. Every book shows its peppers and content details, and everyone's content rules apply (kids only ever see books the AI has rated).

Without a key, the Popular, teen and kids' rows come from Open Library. They work, but they lean toward older books. With a free **New York Times Books API** key you get the real weekly best-seller lists for adults, teens, middle grade and picture books.

## Get the key (about five minutes)

1. Go to [developer.nytimes.com](https://developer.nytimes.com), choose **Create account**, and sign in. It's free.
2. Open your account menu, go to **Apps** (or **My Apps**), and choose **+ New App**.
3. Give it a name, for example *NovelCheck*, switch on the **Books API**, and save.
4. Copy the app's **API key** (the key, not the secret).

## Add it to NovelCheck

1. In NovelCheck, open **Admin → Delivery & Services → Discover**.
2. Paste the key into **New York Times Books API key** and press **Test key**. It should say how many books the teen list has.
3. Press **Save settings**, then **Refresh lists now**. The refresh takes about a minute, because the New York Times asks apps to pause between requests.

## Good to know

- NovelCheck asks for the five lists once a week (they change weekly), well within the free allowance. **Refresh lists now** fetches them straight away.
- The New York Times asks apps to credit them: each row from their lists says "From The New York Times Best Sellers lists · Data provided by The New York Times", each of those books has a **📰 NYT list** tag (with its weeks on the list), and the credit is repeated at the bottom of the tab.
- **Discover books the AI rates per day** (default 30) limits how many new list books are rated each day, best-ranked first, within your AI's hourly token cap. Until a book is rated, parents see "Not rated yet" and kids don't see it.
- Discover books you don't own stay out of your Library. Tap ⭐ **Wishlist** to ask for one, or use the **Amazon** and **Open Library** links. (Kids' accounts don't get the Amazon link.)
- Turn the whole tab off under **Admin → System & Toggles → Features**.
