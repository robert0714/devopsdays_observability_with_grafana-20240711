import http from "k6/http";
import { sleep, check, group } from "k6";

export const options = {
    vus: 5,
    // duration: '180s',
    iterations: 100,
};

export default function () {
    const lunchOptions = [
        "steak",
        "hot_pot",
        "fried_rice",
        "beef_noodle_soup",
        "sushi",
        "tacos",
        "pizza",
        "burger",
        "ramen",
        "salad",
        "sandwich",
    ];
    const weightedDumplings = new Array(10).fill('dumplings');
    const weightedSteak = new Array(6).fill('steak');
    const weightedSushi = new Array(2).fill('fried_rice');

    lunchOptions.push(...weightedDumplings);
    lunchOptions.push(...weightedSteak);
    lunchOptions.push(...weightedSushi);

    const askForLunchPerson = [
        "localhost:8081",
        "localhost:8082",
        "localhost:8083",
        "localhost:8084",
    ]
    const weightedDemo1 = new Array(10).fill('localhost:8081');
    const weightedDemo2 = new Array(5).fill('localhost:8082');
    askForLunchPerson.push(...weightedDemo1);
    askForLunchPerson.push(...weightedDemo2);


    group('main', function () {
        for (let i = 0; i < 50; i++) {
            const randomIndexLunchOption = Math.floor(Math.random() * lunchOptions.length);
            const randomLunchOption = lunchOptions[randomIndexLunchOption];
            const randomIndexPerson = Math.floor(Math.random() * askForLunchPerson.length);
            const randomPerson = askForLunchPerson[randomIndexPerson];
            console.info(`Person ${randomPerson} is asking for lunch option ${randomLunchOption}`);
            const res = http.post(`http://${randomPerson}/lunch-suggestion/${randomLunchOption}?unexpected_issue=true`);

            check(res, {
                'is status 200': (r) => r.status === 200,
            });

            check(res, {
                'is status no error': (r) => r.status < 500,
            });

            sleep(2); // assuming sleep function is defined to pause execution for 5 seconds
        }
    });
}
