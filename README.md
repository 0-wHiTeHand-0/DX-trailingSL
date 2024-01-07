# Trail
Apply automatic trailing stop loss orders to your Darwins.

In each execution, Trail compares the current position and stop-loss order for the given Darwins, and updates the stop-loss dynamically. For example, if you set a trailing stop-loss of 3.5% for a Darwin and after some time it goes up, Trail will move up the stop-loss order in Darwinex to set a difference of 3.5%.

The trailing stop-loss orders are set in the configuration file, and they can be represented as a percentage (if you include a "%") or as an absolut value (if you don't include a "%"). The value you set in the `trailingSL` variable is the difference you want between the current Darwin position and the stop-loss.

Important: Trail only updates existing stop-loss orders. You have to create them manually in Darwinex before using this program.

Important too: Because this program needs to run every time interval, you need a server or a computer that is always on. This program can run in different operating systems, including Linux, MacOS, Windows, *BSD, Android, Solaris, etc, as well as in different architectures, including x86, x86-64, arm, arm64, etc.

## Steps to use it

## Step 1:

Login into Darwinex and go to the [Access to DarwinAPI](https://www.darwinex.com/data/darwin-api). There, generate the access tokens for the environment you want (demo or live). You need the 4 tokens: Access Token, Consumer Key, Consumer Secret, and Refresh Token. Write them in the configuration file.

Important: These tokens are equivalent to your Darwinex password. If someone gets access to them, would be able to invest/disinvest and play with your money the same way as with your password. Protect your config file, and cancel all the access tokens [in Darwinex](https://www.darwinex.com/data/darwin-api) if you stop using this program.

## Step 2:

[Download the binary](https://github.com/0-wHiTeHand-0/DX-trailingSL/releases) and run Trail with the `-i` parameter to list your Darwinex accounts:
```
./trail -i -f config.json
```
This lists your accounts and their respective investorIDs. Choose the investorID of the account you want to use, and add it to the config file.

## Step 3:

Add the Darwin names and trailing stop-loss orders in the configuration file. Make sure that you are already invested in those Darwins, and stop-loss orders are already created for each one.

## Step 4:

Run Trails without the `-i` parameter:
```
./trail -f config.json
```
If the program modifies a stop-loss order, a line like this is shown:
```
Trailing stop-loss order updated for <DarwinName>. New stop-loss value: <Value>
```
If nothing is shown, then nothing was modified. This way you can easily log when Trail make changes in your orders.

## Step 5:

Run automatically the command of step 4 each 10 or 5 minutes (your choice), and try to avoid the weekends (the market is closed, so executing Trail does not make sense). Also, do not exceed the [DarwinAPI limits](https://help.darwinex.com/api-walkthrough#throttling).

You can do this by using Cron in MacOS or Linux systems, or Task Scheduler in Windows. You have a lot of information out there about doing this.

Tip for Linux or MacOSX users (or also, what I do):

A Cron job running each 5 minutes (*/5 * * * *) that launches the following bash script
```
#!/bin/bash
if [ "$(date +%u)" != "7" ] && [ "$(date +%u)" != "6" ]; then
   /PATH/trail -f /PATH/config.json >> /PATH/out.log
fi
```
This small script avoids execution during weekends.
