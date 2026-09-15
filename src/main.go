package main

import "fmt"

func twoSum(nums []int, target int) []int {
    pairs := make(map[int]int)

    for i := range nums {
        num := nums[i]
        complement := target - num
        j, ok := pairs[complement]
        if ok {
            return []int{j, i}
        } else {
            pairs[num] = i
        }
    }

    return nil
}

func main() {
    nums := []int{2 ,7 ,11 ,15}
    target := 9

    fmt.Println(twoSum(nums, target))
}